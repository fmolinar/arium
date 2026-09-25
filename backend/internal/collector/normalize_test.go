package collector

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases scheme and host", "HTTPS://Kubernetes.IO/Blog/Post", "https://kubernetes.io/Blog/Post"},
		{"drops default https port", "https://example.com:443/a", "https://example.com/a"},
		{"drops default http port", "http://example.com:80/a", "http://example.com/a"},
		{"keeps non-default port", "https://example.com:8443/a", "https://example.com:8443/a"},
		{"drops fragment", "https://example.com/a#section", "https://example.com/a"},
		{"drops trailing slash", "https://example.com/blog/", "https://example.com/blog"},
		{"drops root slash", "https://example.com/", "https://example.com"},
		{"drops credentials", "https://user:pass@example.com/a", "https://example.com/a"},
		{"drops tracking params", "https://example.com/a?utm_source=x&UTM_Medium=y&fbclid=z&id=1", "https://example.com/a?id=1"},
		{"sorts query", "https://example.com/a?b=2&a=1", "https://example.com/a?a=1&b=2"},
		{"trims whitespace", "  https://example.com/a  ", "https://example.com/a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalURL(tt.in)
			if err != nil {
				t.Fatalf("CanonicalURL(%q) error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("CanonicalURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalURLRejectsInvalid(t *testing.T) {
	for _, in := range []string{"", "/relative/path", "ftp://example.com/a", "javascript:alert(1)", "https://", "://bad"} {
		if _, err := CanonicalURL(in); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("CanonicalURL(%q) error = %v, want ErrInvalidURL", in, err)
		}
	}
}

func TestArticleIDIsStableAcrossEquivalentURLs(t *testing.T) {
	a, _ := CanonicalURL("https://www.cncf.io/blog/post/?utm_source=rss#top")
	b, _ := CanonicalURL("HTTPS://WWW.CNCF.IO/blog/post")

	if ArticleID(a) != ArticleID(b) {
		t.Errorf("IDs differ for equivalent URLs: %s vs %s", ArticleID(a), ArticleID(b))
	}
	if got := len(ArticleID(a)); got != 16 {
		t.Errorf("ID length = %d, want 16", got)
	}
	if ArticleID(a) == ArticleID("https://www.cncf.io/blog/other") {
		t.Error("different URLs produced the same ID")
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"Tom &amp; Jerry &lt;3", "Tom & Jerry <3"},
		{"  lots\n\n of\t space ", "lots of space"},
		{"<style>p{color:red}</style>Text<script>alert(1)</script>", "Text"},
		{"line<br/>break", "line break"},
	}

	for _, tt := range tests {
		if got := CleanText(tt.in); got != tt.want {
			t.Errorf("CleanText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("short", 10); got != "short" {
		t.Errorf("Truncate kept-short = %q", got)
	}

	got := Truncate("the quick brown fox jumps over the lazy dog", 20)
	if got != "the quick brown fox…" {
		t.Errorf("Truncate at word boundary = %q", got)
	}

	long := strings.Repeat("ü", 50)
	got = Truncate(long, 10)
	if n := utf8.RuneCountInString(got); n != 10 || !utf8.ValidString(got) {
		t.Errorf("Truncate multibyte = %q (%d runes)", got, n)
	}
}

func TestNormalize(t *testing.T) {
	fetched := time.Date(2026, 9, 24, 12, 0, 0, 0, time.FixedZone("CST", -6*3600))
	published := time.Date(2026, 9, 23, 8, 0, 0, 0, time.FixedZone("CEST", 2*3600))

	got, err := Normalize(Article{
		Title:       "  <em>Kubernetes</em> 1.35 released  ",
		Summary:     "<p>Big &amp; shiny.</p>",
		URL:         "https://www.Kubernetes.io/blog/135/?utm_campaign=rss",
		PublishedAt: published,
		Origin:      "kubernetes-blog",
	}, fetched)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}

	if got.Title != "Kubernetes 1.35 released" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Summary != "Big & shiny." {
		t.Errorf("Summary = %q", got.Summary)
	}
	if got.URL != "https://www.kubernetes.io/blog/135" {
		t.Errorf("URL = %q", got.URL)
	}
	if got.ID != ArticleID(got.URL) {
		t.Errorf("ID = %q, want ArticleID(URL)", got.ID)
	}
	if got.Source != "kubernetes.io" {
		t.Errorf("Source = %q, want host without www", got.Source)
	}
	if got.Origin != "kubernetes-blog" {
		t.Errorf("Origin = %q, want it preserved", got.Origin)
	}
	if !got.PublishedAt.Equal(published) || got.PublishedAt.Location() != time.UTC {
		t.Errorf("PublishedAt = %v, want %v in UTC", got.PublishedAt, published)
	}
	if !got.FetchedAt.Equal(fetched) || got.FetchedAt.Location() != time.UTC {
		t.Errorf("FetchedAt = %v, want %v in UTC", got.FetchedAt, fetched)
	}
}

func TestNormalizeDefaults(t *testing.T) {
	fetched := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)

	got, err := Normalize(Article{Title: "t", URL: "https://example.com/a", Source: "Example Blog"}, fetched)
	if err != nil {
		t.Fatalf("Normalize error: %v", err)
	}

	if !got.PublishedAt.Equal(fetched) {
		t.Errorf("PublishedAt = %v, want fetch time when missing", got.PublishedAt)
	}
	if got.Source != "Example Blog" {
		t.Errorf("Source = %q, want explicit source kept", got.Source)
	}
}

func TestNormalizeRejects(t *testing.T) {
	now := time.Now()

	if _, err := Normalize(Article{Title: "<b></b> ", URL: "https://example.com"}, now); !errors.Is(err, ErrMissingTitle) {
		t.Errorf("empty title error = %v, want ErrMissingTitle", err)
	}
	if _, err := Normalize(Article{Title: "t", URL: "not a url"}, now); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("bad URL error = %v, want ErrInvalidURL", err)
	}
}

func TestTitleKey(t *testing.T) {
	tests := []struct {
		a, b string
		same bool
	}{
		{"Kubernetes 1.35: In-Place Resize GA", "kubernetes 1.35 — in-place resize ga!", true},
		{"Show HN: A faster GitOps controller", "A faster GitOps controller", true},
		{"Ask HN: How do you run Argo CD?", "How do you run Argo CD", true},
		{"Kubernetes 1.35 in-place resize GA", "Kubernetes 1.36 in-place resize GA", false},
	}

	for _, tt := range tests {
		ka, kb := TitleKey(tt.a), TitleKey(tt.b)
		if ka == "" || kb == "" {
			t.Errorf("TitleKey(%q)=%q, TitleKey(%q)=%q; want non-empty", tt.a, ka, tt.b, kb)
			continue
		}
		if (ka == kb) != tt.same {
			t.Errorf("TitleKey(%q)=%q vs TitleKey(%q)=%q; same=%v, want %v", tt.a, ka, tt.b, kb, ka == kb, tt.same)
		}
	}

	for _, short := range []string{"v3.2.0", "Release notes", "Community meeting notes", "Show HN: my tool", ""} {
		if key := TitleKey(short); key != "" {
			t.Errorf("TitleKey(%q) = %q, want \"\" for a title too short to compare", short, key)
		}
	}
}
