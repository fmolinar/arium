package collector

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()

	root := filepath.Join(t.TempDir(), "data")

	store, err := NewStore(root)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	return store, root
}

func testRun(id string) Run {
	return Run{ID: id, StartedAt: time.Date(2026, 9, 24, 15, 30, 0, 0, time.UTC)}
}

func article(url string) Article {
	return Article{ID: ArticleID(url), Title: "t " + url, URL: url, Tags: []string{}}
}

func readArticles(t *testing.T, path string) []Article {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	var out []Article
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var a Article
		if err := json.Unmarshal(scanner.Bytes(), &a); err != nil {
			t.Fatalf("decode line %q: %v", scanner.Text(), err)
		}
		out = append(out, a)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}

	return out
}

func TestNewRun(t *testing.T) {
	now := time.Date(2026, 9, 24, 15, 30, 5, 0, time.FixedZone("CST", -6*3600))

	run := NewRun(now)

	if !strings.HasPrefix(run.ID, "20260924T213005Z-") {
		t.Errorf("ID = %q, want UTC timestamp prefix", run.ID)
	}
	if !ValidName(run.ID) {
		t.Errorf("ID %q is not a valid name", run.ID)
	}
	if NewRun(now).ID == run.ID {
		t.Error("two runs at the same instant got the same ID")
	}
}

func TestWriteRaw(t *testing.T) {
	store, root := newTestStore(t)
	run := testRun("run1")

	rel, unchanged, err := store.WriteRaw(run, "kubernetes-blog", Raw{Data: []byte("<rss/>"), Ext: "xml"})
	if err != nil || unchanged {
		t.Fatalf("WriteRaw: %v", err)
	}

	want := filepath.Join("raw", "kubernetes-blog", "2026-09-24", "run1.xml")
	if rel != want {
		t.Errorf("path = %q, want %q", rel, want)
	}

	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil || string(data) != "<rss/>" {
		t.Errorf("content = %q, %v", data, err)
	}
}

func TestWriteRejectsUnsafeNames(t *testing.T) {
	store, root := newTestStore(t)

	for _, name := range []string{"", "../escape", "a/b", ".hidden", "a b"} {
		if _, _, err := store.WriteRaw(testRun("run1"), name, Raw{Ext: "json"}); err == nil {
			t.Errorf("WriteRaw accepted source name %q", name)
		}
		if _, _, err := store.WriteRaw(testRun(name), "src", Raw{Ext: "json"}); err == nil {
			t.Errorf("WriteRaw accepted run ID %q", name)
		}
		if _, _, err := store.WriteRaw(testRun("run1"), "src", Raw{Ext: name}); err == nil {
			t.Errorf("WriteRaw accepted extension %q", name)
		}
		if _, err := store.WriteArticles(testRun(name), nil); err == nil {
			t.Errorf("WriteArticles accepted run ID %q", name)
		}
		if err := store.WriteManifest(Manifest{RunID: name}); err == nil {
			t.Errorf("WriteManifest accepted run ID %q", name)
		}
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape")); !os.IsNotExist(err) {
		t.Error("a file was written outside the store root")
	}
}

func TestWriteArticlesDedupes(t *testing.T) {
	store, root := newTestStore(t)

	a, b, c := article("https://a.test/1"), article("https://b.test/2"), article("https://c.test/3")

	w, err := store.WriteArticles(testRun("run1"), []Article{a, b, a})
	if err != nil {
		t.Fatalf("first WriteArticles: %v", err)
	}
	if w.Written != 2 || w.DuplicatesByURL != 1 {
		t.Errorf("first run = %+v, want 2 written, 1 in-batch URL duplicate", w)
	}
	if want := filepath.Join("articles", "2026-09-24", "run1.ndjson"); w.Path != want {
		t.Errorf("path = %q, want %q", w.Path, want)
	}

	got := readArticles(t, filepath.Join(root, w.Path))
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Errorf("first run file = %+v", got)
	}

	w, err = store.WriteArticles(testRun("run2"), []Article{b, c})
	if err != nil {
		t.Fatalf("second WriteArticles: %v", err)
	}
	if w.Written != 1 || w.DuplicatesByURL != 1 {
		t.Errorf("second run = %+v, want 1 written (b seen in run1)", w)
	}

	got = readArticles(t, filepath.Join(root, w.Path))
	if len(got) != 1 || got[0].ID != c.ID {
		t.Errorf("second run file = %+v", got)
	}
}

func TestWriteArticlesNothingNew(t *testing.T) {
	store, root := newTestStore(t)
	a := article("https://a.test/1")

	if _, err := store.WriteArticles(testRun("run1"), []Article{a}); err != nil {
		t.Fatal(err)
	}

	w, err := store.WriteArticles(testRun("run2"), []Article{a})
	if err != nil || w.Written != 0 || w.Path != "" {
		t.Errorf("WriteArticles = (%+v, %v), want nothing written", w, err)
	}

	if _, err := os.Stat(filepath.Join(root, "articles", "2026-09-24", "run2.ndjson")); !os.IsNotExist(err) {
		t.Error("an empty articles file was written")
	}
}

func TestSeenStatePersistsAcrossStores(t *testing.T) {
	store, root := newTestStore(t)
	a := article("https://a.test/1")

	if _, err := store.WriteArticles(testRun("run1"), []Article{a}); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}

	w, err := reopened.WriteArticles(testRun("run2"), []Article{a})
	if err != nil || w.Written != 0 {
		t.Errorf("reopened store wrote %d (%v), want 0", w.Written, err)
	}
}

func TestCorruptSeenStateFails(t *testing.T) {
	store, root := newTestStore(t)

	if err := os.MkdirAll(filepath.Join(root, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, seenPath), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := store.WriteArticles(testRun("run1"), []Article{article("https://a.test/1")}); err == nil {
		t.Error("WriteArticles succeeded with corrupt seen state; it should refuse rather than re-emit everything")
	}
}

func TestWriteManifest(t *testing.T) {
	store, root := newTestStore(t)

	m := Manifest{
		RunID:           "run1",
		StartedAt:       time.Date(2026, 9, 24, 15, 30, 0, 0, time.UTC),
		FinishedAt:      time.Date(2026, 9, 24, 15, 30, 9, 0, time.UTC),
		Sources:         []SourceResult{{Name: "hackernews", Fetched: 3}, {Name: "cncf-blog", Error: "timeout"}},
		ArticlesWritten: 3,
	}

	if err := store.WriteManifest(m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "runs", "run1.json"))
	if err != nil {
		t.Fatal(err)
	}

	var got Manifest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if got.RunID != "run1" || len(got.Sources) != 2 || got.Sources[1].Error != "timeout" {
		t.Errorf("manifest = %+v", got)
	}
}

func TestWritesLeaveNoTempFiles(t *testing.T) {
	store, root := newTestStore(t)
	run := testRun("run1")

	if _, _, err := store.WriteRaw(run, "src", Raw{Data: []byte("x"), Ext: "json"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteArticles(run, []Article{article("https://a.test/1")}); err != nil {
		t.Fatal(err)
	}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if strings.HasPrefix(d.Name(), ".tmp-") {
			t.Errorf("leftover temp file: %s", path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteArticlesDedupesByTitle(t *testing.T) {
	store, root := newTestStore(t)

	original := Article{ID: "id1", Title: "Kubernetes 1.35: in-place pod resize goes GA", URL: "https://kubernetes.io/blog/135"}
	crossPost := Article{ID: "id2", Title: "Kubernetes 1.35 — In-Place Pod Resize Goes GA!", URL: "https://www.cncf.io/blog/135"}
	showHN := Article{ID: "id3", Title: "Show HN: Kubernetes 1.35 in-place pod resize goes GA", URL: "https://news.ycombinator.com/item?id=1"}
	// Short generic titles are never compared, so both of these are kept.
	release1 := Article{ID: "id4", Title: "v3.2.0", URL: "https://github.com/argoproj/argo-cd/releases/v3.2.0"}
	release2 := Article{ID: "id5", Title: "v3.2.0", URL: "https://github.com/fluxcd/flux2/releases/v3.2.0"}

	w, err := store.WriteArticles(testRun("run1"), []Article{original, crossPost, release1, release2})
	if err != nil {
		t.Fatal(err)
	}
	if w.Written != 3 || w.DuplicatesByTitle != 1 {
		t.Errorf("run1 = %+v, want 3 written, 1 title duplicate", w)
	}

	got := readArticles(t, filepath.Join(root, w.Path))
	if got[0].ID != "id1" {
		t.Errorf("kept %q, want the first occurrence id1", got[0].ID)
	}

	w, err = store.WriteArticles(testRun("run2"), []Article{showHN})
	if err != nil {
		t.Fatal(err)
	}
	if w.Written != 0 || w.DuplicatesByTitle != 1 {
		t.Errorf("run2 = %+v, want the Show HN repost skipped as a title duplicate", w)
	}
}

func TestWriteRawSkipsUnchanged(t *testing.T) {
	store, root := newTestStore(t)
	feed := Raw{Data: []byte("<rss>v1</rss>"), Ext: "xml"}

	if _, unchanged, err := store.WriteRaw(testRun("run1"), "blog", feed); err != nil || unchanged {
		t.Fatalf("first write: unchanged=%v err=%v", unchanged, err)
	}

	path, unchanged, err := store.WriteRaw(testRun("run2"), "blog", feed)
	if err != nil || !unchanged || path != "" {
		t.Errorf("identical payload: path=%q unchanged=%v err=%v, want skipped", path, unchanged, err)
	}
	if _, err := os.Stat(filepath.Join(root, "raw", "blog", "2026-09-24", "run2.xml")); !os.IsNotExist(err) {
		t.Error("unchanged payload was written")
	}

	// Another source with the same bytes is tracked separately.
	if _, unchanged, _ := store.WriteRaw(testRun("run2"), "other", feed); unchanged {
		t.Error("hash state leaked across sources")
	}

	changed := Raw{Data: []byte("<rss>v2</rss>"), Ext: "xml"}
	if path, unchanged, err := store.WriteRaw(testRun("run3"), "blog", changed); err != nil || unchanged || path == "" {
		t.Errorf("changed payload: path=%q unchanged=%v err=%v, want written", path, unchanged, err)
	}
}

func TestLock(t *testing.T) {
	store, root := newTestStore(t)

	unlock, err := store.Lock()
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// A second store on the same root stands in for a second process: flock
	// locks conflict between separate open file descriptions.
	other, _ := NewStore(root)
	if _, err := other.Lock(); !errors.Is(err, ErrLocked) {
		t.Errorf("second Lock err = %v, want ErrLocked", err)
	}

	unlock()

	unlock2, err := other.Lock()
	if err != nil {
		t.Fatalf("Lock after unlock: %v", err)
	}
	unlock2()
}

// seed creates a file under the store root, including parent directories.
func seed(t *testing.T, root, rel string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, rel))
	return err == nil
}

func TestPrune(t *testing.T) {
	store, root := newTestStore(t)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	retention := 30 * 24 * time.Hour // cutoff: 2026-08-25 12:00

	old := []string{
		"raw/blog/2026-08-01/20260801T000000Z-aaaaaa.xml",
		"raw/hackernews/2026-08-24/20260824T160000Z-bbbbbb.json",
		"articles/2026-08-24/20260824T160000Z-bbbbbb.ndjson",
		"runs/20260824T160000Z-bbbbbb.json",
		"runs/20260825T080000Z-cccccc.json", // cutoff day, but before 12:00
	}
	kept := []string{
		"raw/blog/2026-08-25/20260825T160000Z-dddddd.xml", // cutoff day is kept whole
		"raw/blog/2026-09-24/20260924T080000Z-eeeeee.xml",
		"articles/2026-09-24/20260924T080000Z-eeeeee.ndjson",
		"runs/20260825T160000Z-dddddd.json",
		"runs/20260924T080000Z-eeeeee.json",
		"raw/blog/not-a-date/keep.xml", // unexpected names are left alone
	}
	for _, rel := range append(old, kept...) {
		seed(t, root, rel)
	}

	oldTime, newTime := now.AddDate(0, 0, -40), now.AddDate(0, 0, -1)
	mustSave(t, store, seenPath, map[string]time.Time{"old": oldTime, "new": newTime})
	mustSave(t, store, titlesPath, map[string]time.Time{"old title key": oldTime})
	mustSave(t, store, rawPath, map[string]rawState{"blog": {SHA256: "h1", StoredAt: newTime}, "gone": {SHA256: "h2", StoredAt: oldTime}})

	result, err := store.Prune(now, retention)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	for _, rel := range old {
		if exists(root, rel) {
			t.Errorf("%s survived pruning", rel)
		}
	}
	for _, rel := range kept {
		if !exists(root, rel) {
			t.Errorf("%s was pruned", rel)
		}
	}
	if exists(root, "raw/blog/2026-08-01") {
		t.Error("empty day directory left behind")
	}

	if result.Files != len(old) || result.StateEntries != 3 {
		t.Errorf("result = %+v, want %d files and 3 state entries", result, len(old))
	}

	seen := map[string]time.Time{}
	_ = store.loadJSON(seenPath, &seen)
	if _, ok := seen["new"]; !ok || len(seen) != 1 {
		t.Errorf("seen after prune = %v, want only the recent entry", seen)
	}

	raw := map[string]rawState{}
	_ = store.loadJSON(rawPath, &raw)
	if _, ok := raw["gone"]; ok || len(raw) != 1 {
		t.Errorf("raw state after prune = %v, want the old hash dropped", raw)
	}
}

func TestPrunedRawHashIsStoredAgain(t *testing.T) {
	store, _ := newTestStore(t)
	feed := Raw{Data: []byte("<rss/>"), Ext: "xml"}

	old := Run{ID: "run1", StartedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)}
	if _, _, err := store.WriteRaw(old, "blog", feed); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if _, err := store.Prune(now, 30*24*time.Hour); err != nil {
		t.Fatal(err)
	}

	// The only copy was pruned, so the unchanged feed must be kept again.
	if _, unchanged, err := store.WriteRaw(testRun("run2"), "blog", feed); err != nil || unchanged {
		t.Errorf("unchanged=%v err=%v, want a fresh copy after pruning", unchanged, err)
	}
}

func mustSave(t *testing.T, store *Store, rel string, v any) {
	t.Helper()
	if err := store.saveJSON(rel, v); err != nil {
		t.Fatal(err)
	}
}
