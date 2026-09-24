package collector

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// validName guards every caller-supplied path segment (source names, run IDs)
// so nothing can escape the store root.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidName reports whether name can be used as a source name or run ID.
func ValidName(name string) bool {
	return validName.MatchString(name)
}

const runIDTimeLayout = "20060102T150405Z"

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
		ID:        now.Format(runIDTimeLayout) + "-" + hex.EncodeToString(suffix),
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
	Rejected int    `json:"rejected"` // failed normalization (no title, bad URL)
	Stale    int    `json:"stale"`    // older than the collector's MaxAge
	RawPath  string `json:"rawPath,omitempty"`
	// RawUnchanged is set when the payload was identical to the last one
	// stored for this source, so no new raw file was written.
	RawUnchanged bool   `json:"rawUnchanged,omitempty"`
	Error        string `json:"error,omitempty"`
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
	// DuplicatesByURL and DuplicatesByTitle count articles skipped because
	// they were already stored, matched by canonical URL or by title.
	DuplicatesByURL   int          `json:"duplicatesByUrl"`
	DuplicatesByTitle int          `json:"duplicatesByTitle"`
	Pruned            *PruneResult `json:"pruned,omitempty"`
}

// Store writes collector output under a root directory:
//
//	raw/<source>/<YYYY-MM-DD>/<runID>.<ext>  upstream payloads, verbatim, only when changed
//	articles/<YYYY-MM-DD>/<runID>.ndjson     normalized articles new in this run
//	runs/<runID>.json                        run manifests
//	state/seen.json                          article ID → first-seen time
//	state/titles.json                        title key → first-seen time
//	state/raw.json                           source → hash and time of its last stored payload
//	state/.lock                              held for the duration of a run
//
// Every file is written atomically (temp file + rename). Callers must hold
// Lock while writing so concurrent runs can't interleave.
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

// ErrLocked is returned by Lock when another run holds the store.
var ErrLocked = errors.New("another collector run is using this output directory")

// Lock takes an exclusive lock on the store, returning ErrLocked if another
// process holds it. The lock is released by calling the returned function, or
// by the OS if the process dies.
func (s *Store) Lock() (unlock func(), err error) {
	dir := filepath.Join(s.root, "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", dir, err)
	}

	return lockFile(filepath.Join(dir, ".lock"))
}

type rawState struct {
	SHA256   string    `json:"sha256"`
	StoredAt time.Time `json:"storedAt"`
}

const (
	seenPath   = "state/seen.json"
	titlesPath = "state/titles.json"
	rawPath    = "state/raw.json"
)

// WriteRaw stores a source's upstream payload and returns its path relative to
// the store root. If the payload is byte-for-byte identical to the last one
// stored for the source, nothing is written and unchanged is true.
func (s *Store) WriteRaw(run Run, source string, raw Raw) (path string, unchanged bool, err error) {
	if !ValidName(run.ID) || !ValidName(source) || !ValidName(raw.Ext) {
		return "", false, fmt.Errorf("invalid run ID %q, source name %q or extension %q", run.ID, source, raw.Ext)
	}

	state := map[string]rawState{}
	if err := s.loadJSON(rawPath, &state); err != nil {
		return "", false, err
	}

	sum := sha256.Sum256(raw.Data)
	hash := hex.EncodeToString(sum[:])

	if state[source].SHA256 == hash {
		return "", true, nil
	}

	rel := filepath.Join("raw", source, run.day(), run.ID+"."+raw.Ext)
	if err := s.writeFile(rel, raw.Data); err != nil {
		return "", false, err
	}

	state[source] = rawState{SHA256: hash, StoredAt: run.StartedAt.UTC()}

	return rel, false, s.saveJSON(rawPath, state)
}

// ArticlesWrite is the outcome of WriteArticles.
type ArticlesWrite struct {
	Written           int
	Path              string // relative to the store root; "" if nothing was new
	DuplicatesByURL   int
	DuplicatesByTitle int
}

// WriteArticles stores the articles that aren't duplicates of one already
// stored, in a previous run or earlier in this batch. An article is a
// duplicate if its ID (canonical URL) or its TitleKey was seen before.
//
// The articles file is written before the seen-state, so a crash in between
// can make the next run write some articles again, but never loses any.
// Consumers should de-duplicate by ID.
func (s *Store) WriteArticles(run Run, articles []Article) (ArticlesWrite, error) {
	if !ValidName(run.ID) {
		return ArticlesWrite{}, fmt.Errorf("invalid run ID %q", run.ID)
	}

	seen := map[string]time.Time{}
	if err := s.loadJSON(seenPath, &seen); err != nil {
		return ArticlesWrite{}, err
	}

	titles := map[string]time.Time{}
	if err := s.loadJSON(titlesPath, &titles); err != nil {
		return ArticlesWrite{}, err
	}

	var (
		result ArticlesWrite
		buf    bytes.Buffer
	)
	enc := json.NewEncoder(&buf)
	now := run.StartedAt.UTC()

	for _, a := range articles {
		if _, ok := seen[a.ID]; ok {
			result.DuplicatesByURL++
			continue
		}

		key := TitleKey(a.Title)
		if _, ok := titles[key]; ok && key != "" {
			result.DuplicatesByTitle++
			continue
		}

		if err := enc.Encode(a); err != nil {
			return ArticlesWrite{}, fmt.Errorf("encode article %s: %w", a.ID, err)
		}

		seen[a.ID] = now
		if key != "" {
			titles[key] = now
		}
		result.Written++
	}

	if result.Written == 0 {
		return result, nil
	}

	result.Path = filepath.Join("articles", run.day(), run.ID+".ndjson")
	if err := s.writeFile(result.Path, buf.Bytes()); err != nil {
		return ArticlesWrite{}, err
	}

	if err := s.saveJSON(seenPath, seen); err != nil {
		return ArticlesWrite{}, err
	}

	return result, s.saveJSON(titlesPath, titles)
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

// PruneResult counts what Prune removed.
type PruneResult struct {
	Files        int `json:"files"`
	StateEntries int `json:"stateEntries"`
}

// Prune deletes everything older than retention: raw and article files from
// day directories before the cutoff date, manifests of runs that started
// before the cutoff, and state entries first seen before it. A pruned raw hash
// means the next payload from that source is stored again even if unchanged,
// so every source keeps at least one raw copy inside the retention window.
//
// Dropping seen/title entries is only safe while the collector's MaxAge is no
// longer than retention; otherwise old articles could be re-admitted.
func (s *Store) Prune(now time.Time, retention time.Duration) (PruneResult, error) {
	var result PruneResult

	cutoff := now.UTC().Add(-retention)
	cutoffDay := cutoff.Format(time.DateOnly)

	// raw/<source>/<day>/ and articles/<day>/
	rawDirs, err := filepath.Glob(filepath.Join(s.root, "raw", "*", "*"))
	if err != nil {
		return result, err
	}
	articleDirs, err := filepath.Glob(filepath.Join(s.root, "articles", "*"))
	if err != nil {
		return result, err
	}

	for _, dir := range append(rawDirs, articleDirs...) {
		day := filepath.Base(dir)
		if _, err := time.Parse(time.DateOnly, day); err != nil || day >= cutoffDay {
			continue
		}

		n, err := removeDir(dir)
		result.Files += n
		if err != nil {
			return result, err
		}
	}

	manifests, err := filepath.Glob(filepath.Join(s.root, "runs", "*.json"))
	if err != nil {
		return result, err
	}

	for _, path := range manifests {
		stamp, _, _ := strings.Cut(filepath.Base(path), "-")
		started, err := time.Parse(runIDTimeLayout, stamp)
		if err != nil || !started.Before(cutoff) {
			continue
		}

		if err := os.Remove(path); err != nil {
			return result, fmt.Errorf("remove %s: %w", path, err)
		}
		result.Files++
	}

	for _, rel := range []string{seenPath, titlesPath} {
		n, err := s.pruneTimes(rel, cutoff)
		result.StateEntries += n
		if err != nil {
			return result, err
		}
	}

	raw := map[string]rawState{}
	if err := s.loadJSON(rawPath, &raw); err != nil {
		return result, err
	}

	pruned := 0
	for source, st := range raw {
		if st.StoredAt.Before(cutoff) {
			delete(raw, source)
			pruned++
		}
	}

	if pruned > 0 {
		result.StateEntries += pruned
		if err := s.saveJSON(rawPath, raw); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (s *Store) pruneTimes(rel string, cutoff time.Time) (int, error) {
	entries := map[string]time.Time{}
	if err := s.loadJSON(rel, &entries); err != nil {
		return 0, err
	}

	pruned := 0
	for key, at := range entries {
		if at.Before(cutoff) {
			delete(entries, key)
			pruned++
		}
	}

	if pruned == 0 {
		return 0, nil
	}

	return pruned, s.saveJSON(rel, entries)
}

// removeDir deletes dir and returns how many files it contained.
func removeDir(dir string) (int, error) {
	files := 0
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files++
		}
		return nil
	})

	if err := os.RemoveAll(dir); err != nil {
		return 0, fmt.Errorf("remove %s: %w", dir, err)
	}

	return files, nil
}

// loadJSON decodes the state file rel into v, leaving v untouched if the file
// doesn't exist yet. A corrupt file is an error rather than an empty state, so
// a bad write can't silently re-emit everything.
func (s *Store) loadJSON(rel string, v any) error {
	data, err := os.ReadFile(filepath.Join(s.root, rel))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("decode %s: %w", rel, err)
	}

	return nil
}

func (s *Store) saveJSON(rel string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("encode %s: %w", rel, err)
	}

	return s.writeFile(rel, data)
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
