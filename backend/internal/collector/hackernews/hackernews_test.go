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

	if src.cfg.BaseURL != "https://hn.algolia.com/api/v1" || src.cfg.MinPoints != 20 || src.cfg.HitsPerQuery != 30 || src.cfg.Now == nil ||
		src.cfg.QueryTimeout != 20*time.Second || src.cfg.RetryDelay != time.Second {
		t.Errorf("defaults = %+v", src.cfg)
	}
}

func TestFetchKeepsResultsOfQueriesThatSucceed(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	src := New(Config{
		Queries:    []Query{{Query: "kubernetes"}, {Query: "missing-fixture"}},
		BaseURL:    srv.URL,
		RetryDelay: time.Millisecond,
	}, srv.Client())

	raw, articles, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v, want the kubernetes results despite the failed query", err)
	}
	if len(articles) != 2 {
		t.Errorf("got %d articles, want the 2 from kubernetes", len(articles))
	}

	var stored map[string]json.RawMessage
	if err := json.Unmarshal(raw.Data, &stored); err != nil || len(stored) != 1 || stored["kubernetes"] == nil {
		t.Errorf("raw payload keys = %v (err %v), want only kubernetes", slices.Sorted(maps.Keys(stored)), err)
	}

	attempts := 0
	for _, r := range fake.requests {
		if r.Get("query") == "missing-fixture" {
			attempts++
		}
	}
	if attempts != maxAttempts {
		t.Errorf("failed query was tried %d times, want %d", attempts, maxAttempts)
	}
}

func TestFetchFailsIfEveryQueryFails(t *testing.T) {
	fake := &fakeAlgolia{}
	srv := fake.start(t)

	src := New(Config{
		Queries:    []Query{{Query: "missing-fixture"}, {Query: "also-missing"}},
		BaseURL:    srv.URL,
		RetryDelay: time.Millisecond,
	}, srv.Client())

	_, _, err := src.Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "all 2 queries failed") {
		t.Errorf("err = %v, want every query reported as failed", err)
	}
}

func TestFetchRetriesQueryThatTimesOut(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		first := calls == 1
		mu.Unlock()

		if first {
			<-r.Context().Done() // hang until the attempt times out
			return
		}
		_, _ = w.Write([]byte(`{"hits":[{"objectID":"1","title":"Kubernetes 2.0","url":"https://example.com/k2"}]}`))
	}))
	t.Cleanup(srv.Close)

	src := New(Config{
		Queries:      []Query{{Query: "kubernetes"}},
		BaseURL:      srv.URL,
		QueryTimeout: 50 * time.Millisecond,
		RetryDelay:   time.Millisecond,
	}, srv.Client())

	_, articles, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v, want the retry to succeed", err)
	}
	if len(articles) != 1 || calls != 2 {
		t.Errorf("got %d articles after %d calls, want 1 after 2", len(articles), calls)
	}
}

func TestFetchDoesNotRetryAfterSourceDeadline(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	src := New(Config{
		Queries:      []Query{{Query: "kubernetes"}},
		BaseURL:      srv.URL,
		QueryTimeout: time.Minute,
		RetryDelay:   time.Millisecond,
	}, srv.Client())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, _, err := src.Fetch(ctx); err == nil {
		t.Fatal("Fetch succeeded against a server that never answers")
	}
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Errorf("server got %d calls, want 1 (no retry once the source's deadline passed)", calls)
	}
}

func TestFetchRejectsInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>rate limited</html>"))
	}))
	t.Cleanup(srv.Close)

	src := New(Config{Queries: []Query{{Query: "x"}}, BaseURL: srv.URL, RetryDelay: time.Millisecond}, srv.Client())
	if _, _, err := src.Fetch(context.Background()); err == nil {
		t.Error("Fetch succeeded on a non-JSON body")
	}
}

func TestFetchRunsQueriesConcurrently(t *testing.T) {
	var mu sync.Mutex
	inFlight, peak := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()

		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		inFlight--
		mu.Unlock()
		_, _ = w.Write([]byte(`{"hits":[]}`))
	}))
	t.Cleanup(srv.Close)

	var queries []Query
	for _, q := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		queries = append(queries, Query{Query: q})
	}
	src := New(Config{Queries: queries, BaseURL: srv.URL}, srv.Client())

	raw, _, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	var responses map[string]json.RawMessage
	if err := json.Unmarshal(raw.Data, &responses); err != nil || len(responses) != len(queries) {
		t.Errorf("raw payload has %d responses (err %v), want %d", len(responses), err, len(queries))
	}
	if peak < 2 || peak > maxConcurrent {
		t.Errorf("peak concurrent queries = %d, want 2..%d", peak, maxConcurrent)
	}
}
