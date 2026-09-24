package hackernews

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAlgolia serves testdata/<query>.json for /search_by_date and records the
// query parameters of each request.
type fakeAlgolia struct {
	mu       sync.Mutex
	requests []url.Values
}

func (f *fakeAlgolia) start(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search_by_date" {
			http.NotFound(w, r)
			return
		}

		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Query())
		f.mu.Unlock()

		data, err := os.ReadFile("testdata/" + r.URL.Query().Get("query") + ".json")
		if err != nil {
			http.Error(w, "no fixture", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestFetch(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	src := New(Config{
		Queries: []Query{
			{Query: "kubernetes", Tags: []string{"devops"}},
			{Query: "gitops", Tags: []string{"gitops"}},
		},
		BaseURL: srv.URL,
	}, srv.Client())

	raw, articles, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if src.Name() != "hackernews" {
		t.Errorf("Name = %q", src.Name())
	}

	// 1002 is returned by both queries but must appear once.
	if len(articles) != 3 {
		t.Fatalf("got %d articles, want 3: %+v", len(articles), articles)
	}

	probes, askHN, noTime := articles[0], articles[1], articles[2]

	if probes.URL != "https://blog.example.com/probes" || probes.Title != "How Kubernetes probes work" {
		t.Errorf("probes = %+v", probes)
	}
	if probes.Summary != "Discussed on Hacker News: 143 points, 25 comments." {
		t.Errorf("fallback summary = %q", probes.Summary)
	}
	if want := time.Unix(1790150400, 0); !probes.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", probes.PublishedAt, want)
	}

	if askHN.URL != "https://news.ycombinator.com/item?id=1002" {
		t.Errorf("Ask HN URL = %q, want the discussion page", askHN.URL)
	}
	if !strings.Contains(askHN.Summary, "300 clusters") {
		t.Errorf("Ask HN summary = %q, want story text", askHN.Summary)
	}
	if !slices.Equal(askHN.Tags, []string{"devops", "gitops"}) {
		t.Errorf("Ask HN tags = %v, want tags from both queries", askHN.Tags)
	}

	if !noTime.PublishedAt.IsZero() {
		t.Errorf("missing created_at_i gave %v, want zero (Normalize fills in fetch time)", noTime.PublishedAt)
	}

	var stored map[string]json.RawMessage
	if err := json.Unmarshal(raw.Data, &stored); err != nil {
		t.Fatalf("raw payload is not JSON: %v", err)
	}
	if raw.Ext != "json" || len(stored) != 2 || stored["kubernetes"] == nil || stored["gitops"] == nil {
		t.Errorf("raw payload keys = %v, want one per query", slices.Sorted(maps.Keys(stored)))
	}
}

func TestSearchParameters(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	src := New(Config{Queries: []Query{{Query: "kubernetes"}}, MinPoints: 50, BaseURL: srv.URL}, srv.Client())
	if _, _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}

	got := fake.requests[0]
	want := map[string]string{
		"query":                        "kubernetes",
		"tags":                         "story",
		"restrictSearchableAttributes": "title",
		"numericFilters":               "points>=50",
		"hitsPerPage":                  "30",
	}
	for key, value := range want {
		if got.Get(key) != value {
			t.Errorf("%s = %q, want %q", key, got.Get(key), value)
		}
	}
}

func TestSearchParametersMaxAge(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	now := time.Unix(1790200000, 0)
	src := New(Config{
		Queries: []Query{{Query: "kubernetes"}},
		MaxAge:  24 * time.Hour,
		BaseURL: srv.URL,
		Now:     func() time.Time { return now },
	}, srv.Client())
	if _, _, err := src.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got, want := fake.requests[0].Get("numericFilters"), "points>=20,created_at_i>1790113600"; got != want {
		t.Errorf("numericFilters = %q, want %q", got, want)
	}
}

func TestDefaults(t *testing.T) {
	src := New(Config{}, http.DefaultClient)

	if src.cfg.BaseURL != "https://hn.algolia.com/api/v1" || src.cfg.MinPoints != 20 || src.cfg.HitsPerQuery != 30 || src.cfg.Now == nil {
		t.Errorf("defaults = %+v", src.cfg)
	}
}

func TestFetchFailsIfAnyQueryFails(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	src := New(Config{
		Queries: []Query{{Query: "kubernetes"}, {Query: "missing-fixture"}},
		BaseURL: srv.URL,
	}, srv.Client())

	_, _, err := src.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), `"missing-fixture"`) {
		t.Errorf("err = %v, want failure naming the query", err)
	}
}

func TestFetchRejectsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>rate limited</html>"))
	}))
	t.Cleanup(srv.Close)

	src := New(Config{Queries: []Query{{Query: "x"}}, BaseURL: srv.URL}, srv.Client())
	if _, _, err := src.Fetch(context.Background()); err == nil {
		t.Error("Fetch succeeded on a non-JSON body")
	}
}
