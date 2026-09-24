package collector

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeSource struct {
	name     string
	raw      Raw
	articles []Article
	err      error
	block    bool // wait for ctx cancellation instead of returning
	panics   bool
}

func (f *fakeSource) Name() string { return f.name }

func (f *fakeSource) Fetch(ctx context.Context) (Raw, []Article, error) {
	if f.panics {
		panic("boom")
	}
	if f.block {
		<-ctx.Done()
		return Raw{}, nil, ctx.Err()
	}
	return f.raw, f.articles, f.err
}

var collectNow = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

func newCollector(sources ...Source) *Collector {
	return &Collector{
		Sources: sources,
		Tagger:  DefaultTagger(),
		Now:     func() time.Time { return collectNow },
	}
}

func TestCollect(t *testing.T) {
	blog := &fakeSource{
		name: "blog",
		raw:  Raw{Data: []byte("<rss/>"), Ext: "xml"},
		articles: []Article{
			{Title: "Older Kubernetes post", URL: "https://blog.test/old", PublishedAt: collectNow.Add(-48 * time.Hour)},
			{Title: "Newer GitOps post", URL: "https://blog.test/new?utm_source=x", PublishedAt: collectNow.Add(-time.Hour)},
			{Title: "", URL: "https://blog.test/untitled"},
			{Title: "Bad URL", URL: "not-a-url"},
		},
	}
	hn := &fakeSource{
		name: "hn",
		raw:  Raw{Data: []byte("{}"), Ext: "json"},
		articles: []Article{
			// Same article as the blog's, via a different-looking URL.
			{Title: "Newer GitOps post (HN)", URL: "https://BLOG.test/new/", PublishedAt: collectNow.Add(-time.Hour)},
		},
	}
	broken := &fakeSource{name: "broken", err: errors.New("connection refused")}

	result, err := newCollector(blog, hn, broken).Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(result.Articles) != 2 {
		t.Fatalf("got %d articles, want 2: %+v", len(result.Articles), result.Articles)
	}

	newest, older := result.Articles[0], result.Articles[1]
	if newest.Title != "Newer GitOps post" || older.Title != "Older Kubernetes post" {
		t.Errorf("order = %q, %q; want newest first, first source wins on duplicates", newest.Title, older.Title)
	}
	if newest.Origin != "blog" {
		t.Errorf("Origin = %q, want source name", newest.Origin)
	}
	if len(newest.Tags) != 1 || newest.Tags[0] != TopicGitOps {
		t.Errorf("Tags = %v, want [gitops]", newest.Tags)
	}

	want := []SourceResult{
		{Name: "blog", Fetched: 4, Rejected: 2},
		{Name: "hn", Fetched: 1},
		{Name: "broken", Error: "connection refused"},
	}
	for i, w := range want {
		if result.Sources[i] != w {
			t.Errorf("Sources[%d] = %+v, want %+v", i, result.Sources[i], w)
		}
	}
}

func TestCollectMaxAge(t *testing.T) {
	src := &fakeSource{name: "blog", articles: []Article{
		{Title: "fresh", URL: "https://a.test/1", PublishedAt: collectNow.Add(-time.Hour)},
		{Title: "stale", URL: "https://a.test/2", PublishedAt: collectNow.Add(-100 * time.Hour)},
	}}

	c := newCollector(src)
	c.MaxAge = 72 * time.Hour

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Articles) != 1 || result.Articles[0].Title != "fresh" {
		t.Errorf("articles = %+v, want only the fresh one", result.Articles)
	}
	if result.Sources[0].Stale != 1 {
		t.Errorf("Stale = %d, want 1", result.Sources[0].Stale)
	}
}

func TestCollectTimeoutAndPanicAreIsolated(t *testing.T) {
	slow := &fakeSource{name: "slow", block: true}
	crashy := &fakeSource{name: "crashy", panics: true}
	ok := &fakeSource{name: "ok", articles: []Article{{Title: "t", URL: "https://a.test/1"}}}

	c := newCollector(slow, crashy, ok)
	c.SourceTimeout = 50 * time.Millisecond

	result, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if result.Sources[0].Error == "" || result.Sources[1].Error == "" {
		t.Errorf("sources = %+v, want errors for slow and crashy", result.Sources)
	}
	if len(result.Articles) != 1 {
		t.Errorf("got %d articles, want 1 from the healthy source", len(result.Articles))
	}
}

func TestCollectAllFailed(t *testing.T) {
	c := newCollector(&fakeSource{name: "a", err: errors.New("x")}, &fakeSource{name: "b", err: errors.New("y")})

	if _, err := c.Collect(context.Background()); !errors.Is(err, ErrAllSourcesFailed) {
		t.Errorf("err = %v, want ErrAllSourcesFailed", err)
	}
}

func TestPersist(t *testing.T) {
	store, root := newTestStore(t)

	good := &fakeSource{
		name:     "good",
		raw:      Raw{Data: []byte("<rss/>"), Ext: "xml"},
		articles: []Article{{Title: "Kubernetes news", URL: "https://a.test/1"}},
	}
	failed := &fakeSource{name: "failed", err: errors.New("timeout")}

	result, err := newCollector(good, failed).Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	manifest, err := Persist(store, result, collectNow.Add(5*time.Second), 0)
	if err != nil {
		t.Fatalf("Persist: %v", err)
	}

	if manifest.ArticlesWritten != 1 || manifest.ArticlesPath == "" {
		t.Errorf("manifest = %+v", manifest)
	}
	if manifest.Sources[0].RawPath == "" || manifest.Sources[1].RawPath != "" {
		t.Errorf("raw paths = %q, %q; want only the good source's", manifest.Sources[0].RawPath, manifest.Sources[1].RawPath)
	}
	if _, err := os.Stat(filepath.Join(root, manifest.Sources[0].RawPath)); err != nil {
		t.Errorf("raw file missing: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "runs", result.Run.ID+".json"))
	if err != nil {
		t.Fatalf("manifest file: %v", err)
	}

	var onDisk Manifest
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Sources[1].Error != "timeout" || !onDisk.FinishedAt.Equal(collectNow.Add(5*time.Second)) {
		t.Errorf("manifest on disk = %+v", onDisk)
	}
}

func TestPersistSkipsUnchangedRawAndPrunes(t *testing.T) {
	store, root := newTestStore(t)
	src := &fakeSource{
		name:     "blog",
		raw:      Raw{Data: []byte("<rss/>"), Ext: "xml"},
		articles: []Article{{Title: "Kubernetes news for this week", URL: "https://a.test/1"}},
	}

	// An old run's leftovers, due for pruning.
	seed(t, root, "raw/blog/2026-08-01/20260801T000000Z-aaaaaa.xml")
	seed(t, root, "runs/20260801T000000Z-aaaaaa.json")

	c := newCollector(src)
	retention := 30 * 24 * time.Hour

	first, _ := c.Collect(context.Background())
	m1, err := Persist(store, first, collectNow, retention)
	if err != nil {
		t.Fatalf("first Persist: %v", err)
	}
	if m1.Sources[0].RawUnchanged || m1.Sources[0].RawPath == "" || m1.ArticlesWritten != 1 {
		t.Errorf("first manifest = %+v", m1)
	}
	if m1.Pruned == nil || m1.Pruned.Files != 2 {
		t.Errorf("Pruned = %+v, want the 2 old files", m1.Pruned)
	}

	second, _ := c.Collect(context.Background())
	m2, err := Persist(store, second, collectNow.Add(8*time.Hour), retention)
	if err != nil {
		t.Fatalf("second Persist: %v", err)
	}
	if !m2.Sources[0].RawUnchanged || m2.Sources[0].RawPath != "" {
		t.Errorf("second run source = %+v, want raw skipped as unchanged", m2.Sources[0])
	}
	if m2.ArticlesWritten != 0 || m2.DuplicatesByURL != 1 {
		t.Errorf("second manifest = %+v, want the article counted as a duplicate", m2)
	}
}

func TestPersistWithoutRetentionDoesNotPrune(t *testing.T) {
	store, root := newTestStore(t)
	seed(t, root, "raw/blog/2020-01-01/20200101T000000Z-aaaaaa.xml")

	result, _ := newCollector(&fakeSource{name: "blog"}).Collect(context.Background())
	m, err := Persist(store, result, collectNow, 0)
	if err != nil {
		t.Fatal(err)
	}

	if m.Pruned != nil || !exists(root, "raw/blog/2020-01-01/20200101T000000Z-aaaaaa.xml") {
		t.Errorf("retention 0 pruned data: %+v", m.Pruned)
	}
}

func TestPersistLocked(t *testing.T) {
	store, root := newTestStore(t)

	unlock, err := store.Lock()
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()

	other, _ := NewStore(root)
	result, _ := newCollector(&fakeSource{name: "blog"}).Collect(context.Background())

	if _, err := Persist(other, result, collectNow, 0); !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
	if exists(root, filepath.Join("runs", result.Run.ID+".json")) {
		t.Error("manifest written without the lock")
	}
}
