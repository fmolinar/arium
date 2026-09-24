package collector

import (
	"bufio"
	"encoding/json"
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

	rel, err := store.WriteRaw(run, "kubernetes-blog", Raw{Data: []byte("<rss/>"), Ext: "xml"})
	if err != nil {
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
		if _, err := store.WriteRaw(testRun("run1"), name, Raw{Ext: "json"}); err == nil {
			t.Errorf("WriteRaw accepted source name %q", name)
		}
		if _, err := store.WriteRaw(testRun(name), "src", Raw{Ext: "json"}); err == nil {
			t.Errorf("WriteRaw accepted run ID %q", name)
		}
		if _, err := store.WriteRaw(testRun("run1"), "src", Raw{Ext: name}); err == nil {
			t.Errorf("WriteRaw accepted extension %q", name)
		}
		if _, _, err := store.WriteArticles(testRun(name), nil); err == nil {
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

	n, rel, err := store.WriteArticles(testRun("run1"), []Article{a, b, a})
	if err != nil {
		t.Fatalf("first WriteArticles: %v", err)
	}
	if n != 2 {
		t.Errorf("first run wrote %d, want 2 (in-batch duplicate dropped)", n)
	}
	if want := filepath.Join("articles", "2026-09-24", "run1.ndjson"); rel != want {
		t.Errorf("path = %q, want %q", rel, want)
	}

	got := readArticles(t, filepath.Join(root, rel))
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID {
		t.Errorf("first run file = %+v", got)
	}

	n, rel, err = store.WriteArticles(testRun("run2"), []Article{b, c})
	if err != nil {
		t.Fatalf("second WriteArticles: %v", err)
	}
	if n != 1 {
		t.Errorf("second run wrote %d, want 1 (b seen in run1)", n)
	}

	got = readArticles(t, filepath.Join(root, rel))
	if len(got) != 1 || got[0].ID != c.ID {
		t.Errorf("second run file = %+v", got)
	}
}

func TestWriteArticlesNothingNew(t *testing.T) {
	store, root := newTestStore(t)
	a := article("https://a.test/1")

	if _, _, err := store.WriteArticles(testRun("run1"), []Article{a}); err != nil {
		t.Fatal(err)
	}

	n, rel, err := store.WriteArticles(testRun("run2"), []Article{a})
	if err != nil || n != 0 || rel != "" {
		t.Errorf("WriteArticles = (%d, %q, %v), want (0, \"\", nil)", n, rel, err)
	}

	if _, err := os.Stat(filepath.Join(root, "articles", "2026-09-24", "run2.ndjson")); !os.IsNotExist(err) {
		t.Error("an empty articles file was written")
	}
}

func TestSeenStatePersistsAcrossStores(t *testing.T) {
	store, root := newTestStore(t)
	a := article("https://a.test/1")

	if _, _, err := store.WriteArticles(testRun("run1"), []Article{a}); err != nil {
		t.Fatal(err)
	}

	reopened, err := NewStore(root)
	if err != nil {
		t.Fatal(err)
	}

	n, _, err := reopened.WriteArticles(testRun("run2"), []Article{a})
	if err != nil || n != 0 {
		t.Errorf("reopened store wrote %d (%v), want 0", n, err)
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

	if _, _, err := store.WriteArticles(testRun("run1"), []Article{article("https://a.test/1")}); err == nil {
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

	if _, err := store.WriteRaw(run, "src", Raw{Data: []byte("x"), Ext: "json"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.WriteArticles(run, []Article{article("https://a.test/1")}); err != nil {
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
