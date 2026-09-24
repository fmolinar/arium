package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// serveFile serves a testdata file and records the last request's User-Agent.
func serveFile(t *testing.T, name string, userAgent *string) *httptest.Server {
	t.Helper()

	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if userAgent != nil {
			*userAgent = r.UserAgent()
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestFetchRSS2(t *testing.T) {
	var ua string
	srv := serveFile(t, "rss2.xml", &ua)

	src := New(Feed{Name: "example-blog", URL: srv.URL, Tags: []string{"devops"}}, srv.Client())

	raw, articles, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if src.Name() != "example-blog" {
		t.Errorf("Name = %q", src.Name())
	}
	if raw.Ext != "xml" || !strings.Contains(string(raw.Data), "<rss") {
		t.Errorf("raw = %q / %d bytes, want the verbatim XML", raw.Ext, len(raw.Data))
	}
	if !strings.HasPrefix(ua, "arium-collector/") {
		t.Errorf("User-Agent = %q", ua)
	}

	// The untitled item is still returned; rejecting it is Normalize's job.
	if len(articles) != 3 {
		t.Fatalf("got %d articles, want 3", len(articles))
	}

	first := articles[0]
	if first.Title != "Kubernetes 1.35: in-place resize goes GA" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.URL != "https://blog.example.com/2026/09/k8s-135/?utm_source=rss" {
		t.Errorf("URL = %q", first.URL)
	}
	if !strings.Contains(first.Summary, "resized") {
		t.Errorf("Summary = %q", first.Summary)
	}
	if want := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC); !first.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", first.PublishedAt, want)
	}
	if !slices.Equal(first.Tags, []string{"devops"}) {
		t.Errorf("Tags = %v, want feed tags", first.Tags)
	}

	// Each article must get its own copy of the feed tags.
	articles[0].Tags[0] = "mutated"
	if articles[1].Tags[0] != "devops" {
		t.Error("articles share a tags slice")
	}
}

func TestFetchAtomFallsBackToContentAndUpdated(t *testing.T) {
	srv := serveFile(t, "atom.xml", nil)

	_, articles, err := New(Feed{Name: "example-releases", URL: srv.URL}, srv.Client()).Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(articles) != 1 {
		t.Fatalf("got %d articles, want 1", len(articles))
	}

	a := articles[0]
	if a.URL != "https://git.example.com/project/releases/tag/v3.2.0" {
		t.Errorf("URL = %q", a.URL)
	}
	if !strings.Contains(a.Summary, "OCI Helm chart") {
		t.Errorf("Summary = %q, want content when there is no description", a.Summary)
	}
	if want := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC); !a.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want updated time %v", a.PublishedAt, want)
	}
}

func TestFetchErrors(t *testing.T) {
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)

	garbage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not a feed</html>"))
	}))
	t.Cleanup(garbage.Close)

	for name, url := range map[string]string{"404": notFound.URL, "not a feed": garbage.URL} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := New(Feed{Name: "x", URL: url}, http.DefaultClient).Fetch(context.Background()); err == nil {
				t.Error("Fetch succeeded, want error")
			}
		})
	}
}
