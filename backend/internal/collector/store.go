package collector

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// validName guards every caller-supplied path segment (source names, run IDs)
// so nothing can escape the store root.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidName reports whether name can be used as a source name or run ID.
func ValidName(name string) bool {
	return validName.MatchString(name)
}

// Run identifies one collector execution. Every file it writes is partitioned
// by the run's start date and tagged with its ID.
type Run struct {
	ID        string
	StartedAt time.Time
}

// NewRun returns a Run with a sortable, collision-resistant ID such as
// "20260924T153000Z-a1b2c3".
func NewRun(now time.Time) Run {
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)

	now = now.UTC()

	return Run{
		ID:        now.Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix),
		StartedAt: now,
	}
}

func (r Run) day() string {
	return r.StartedAt.UTC().Format(time.DateOnly)
}

// SourceResult is one source's outcome within a run.
type SourceResult struct {
	Name     string `json:"name"`
	Fetched  int    `json:"fetched"`
	Rejected int    `json:"rejected"`
	RawPath  string `json:"rawPath,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Manifest summarizes a run and is written last, so its presence means the
// run finished.
type Manifest struct {
	RunID           string         `json:"runId"`
	StartedAt       time.Time      `json:"startedAt"`
	FinishedAt      time.Time      `json:"finishedAt"`
	Sources         []SourceResult `json:"sources"`
	ArticlesWritten int            `json:"articlesWritten"`
	ArticlesPath    string         `json:"articlesPath,omitempty"`
}

// Store writes collector output under a root directory:
//
//	raw/<source>/<YYYY-MM-DD>/<runID>.json   upstream payloads, verbatim
//	articles/<YYYY-MM-DD>/<runID>.ndjson     normalized articles new in this run
//	state/seen.json                          article ID → first-seen time
//	runs/<runID>.json                        run manifests
//
// Every file is written atomically (temp file + rename). A Store is not safe
// for concurrent runs against the same root.
type Store struct {
	root string
}

// NewStore creates root if needed and returns a Store writing into it.
func NewStore(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create store root: %w", err)
	}

	return &Store{root: root}, nil
}

// WriteRaw stores a source's upstream payload and returns its path relative to
// the store root.
func (s *Store) WriteRaw(run Run, source string, data []byte) (string, error) {
	if !ValidName(run.ID) || !ValidName(source) {
		return "", fmt.Errorf("invalid run ID %q or source name %q", run.ID, source)
	}

	rel := filepath.Join("raw", source, run.day(), run.ID+".json")

	return rel, s.writeFile(rel, data)
}

// WriteArticles stores the articles not seen in any previous run, and returns
// how many were written and the relative path of the file ("" if none were
// new). Duplicates within the batch are dropped too.
//
// The articles file is written before the seen-state, so a crash in between
// can make the next run write some articles again, but never loses any.
// Consumers should de-duplicate by ID.
func (s *Store) WriteArticles(run Run, articles []Article) (int, string, error) {
	if !ValidName(run.ID) {
		return 0, "", fmt.Errorf("invalid run ID %q", run.ID)
	}

	seen, err := s.loadSeen()
	if err != nil {
		return 0, "", err
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	written := 0

	for _, a := range articles {
		if _, ok := seen[a.ID]; ok {
			continue
		}

		if err := enc.Encode(a); err != nil {
			return 0, "", fmt.Errorf("encode article %s: %w", a.ID, err)
		}

		seen[a.ID] = run.StartedAt.UTC()
		written++
	}

	if written == 0 {
		return 0, "", nil
	}

	rel := filepath.Join("articles", run.day(), run.ID+".ndjson")
	if err := s.writeFile(rel, buf.Bytes()); err != nil {
		return 0, "", err
	}

	if err := s.saveSeen(seen); err != nil {
		return 0, "", err
	}

	return written, rel, nil
}

// WriteManifest stores the run summary.
func (s *Store) WriteManifest(m Manifest) error {
	if !ValidName(m.RunID) {
		return fmt.Errorf("invalid run ID %q", m.RunID)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}

	return s.writeFile(filepath.Join("runs", m.RunID+".json"), append(data, '\n'))
}

const seenPath = "state/seen.json"

func (s *Store) loadSeen() (map[string]time.Time, error) {
	seen := map[string]time.Time{}

	data, err := os.ReadFile(filepath.Join(s.root, seenPath))
	if errors.Is(err, fs.ErrNotExist) {
		return seen, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read seen state: %w", err)
	}

	if err := json.Unmarshal(data, &seen); err != nil {
		return nil, fmt.Errorf("decode seen state: %w", err)
	}

	return seen, nil
}

func (s *Store) saveSeen(seen map[string]time.Time) error {
	data, err := json.Marshal(seen)
	if err != nil {
		return fmt.Errorf("encode seen state: %w", err)
	}

	return s.writeFile(seenPath, data)
}

// writeFile atomically writes data to rel under the store root: readers see
// either the old file or the complete new one, never a partial write.
func (s *Store) writeFile(rel string, data []byte) error {
	path := filepath.Join(s.root, rel)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", rel, err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", rel, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", rel, err)
	}

	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return fmt.Errorf("chmod %s: %w", rel, err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("rename into %s: %w", rel, err)
	}

	return nil
}
