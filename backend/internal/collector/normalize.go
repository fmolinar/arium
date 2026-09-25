package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxTitleLen   = 200
	maxSummaryLen = 400
)

var (
	ErrMissingTitle = errors.New("article has no title")
	ErrInvalidURL   = errors.New("article URL must be an absolute http(s) URL")
)

var (
	scriptOrStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTag       = regexp.MustCompile(`<[^>]*>`)
	whitespace    = regexp.MustCompile(`\s+`)
)

// trackingParams are query parameters that never change which page a URL
// points at, so they are dropped before hashing to keep IDs stable.
var trackingParams = map[string]bool{
	"fbclid": true,
	"gclid":  true,
	"mc_cid": true,
	"mc_eid": true,
}

// Normalize cleans an article from a source into its stored form: canonical
// URL, stable ID, plain-text title and summary, UTC timestamps and a Source
// host. Tags are left untouched; see Tagger.
func Normalize(a Article, fetchedAt time.Time) (Article, error) {
	canonical, err := CanonicalURL(a.URL)
	if err != nil {
		return Article{}, err
	}

	a.URL = canonical
	a.ID = ArticleID(canonical)

	a.Title = Truncate(CleanText(a.Title), maxTitleLen)
	if a.Title == "" {
		return Article{}, ErrMissingTitle
	}

	a.Summary = Truncate(CleanText(a.Summary), maxSummaryLen)

	if a.Source == "" {
		u, _ := url.Parse(canonical)
		a.Source = strings.TrimPrefix(u.Hostname(), "www.")
	}

	a.FetchedAt = fetchedAt.UTC()
	if a.PublishedAt.IsZero() {
		a.PublishedAt = a.FetchedAt
	}
	a.PublishedAt = a.PublishedAt.UTC()

	return a, nil
}

// CanonicalURL reduces equivalent URLs to one form: lowercase scheme and
// host, no default port, no fragment, no tracking parameters, sorted query
// and no trailing slash.
func CanonicalURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", fmt.Errorf("%w: %q", ErrInvalidURL, raw)
	}

	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	u.Host = host

	u.User = nil
	u.Fragment = ""
	u.RawFragment = ""

	query := u.Query()
	for key := range query {
		if strings.HasPrefix(strings.ToLower(key), "utm_") || trackingParams[strings.ToLower(key)] {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()

	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""

	return u.String(), nil
}

// ArticleID derives a short, stable ID from a canonical URL.
func ArticleID(canonicalURL string) string {
	sum := sha256.Sum256([]byte(canonicalURL))
	return hex.EncodeToString(sum[:8])
}

// CleanText turns feed HTML into a single line of plain text.
func CleanText(s string) string {
	s = scriptOrStyle.ReplaceAllString(s, " ")
	s = htmlTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)

	return strings.TrimSpace(whitespace.ReplaceAllString(s, " "))
}

// Truncate shortens s to at most max runes, cutting at a word boundary where
// possible and marking the cut with an ellipsis.
func Truncate(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	runes := []rune(s)
	cut := string(runes[:max-1])

	// Back up to the last space unless the cut already ends a word.
	if runes[max-1] != ' ' {
		if i := strings.LastIndex(cut, " "); i > len(cut)/2 {
			cut = cut[:i]
		}
	}

	return strings.TrimRight(cut, " .,;:") + "…"
}

var nonAlphanumeric = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// hnPrefixes are stripped so "Show HN: X" matches a blog post titled "X".
var hnPrefixes = []string{"show hn ", "ask hn ", "launch hn ", "tell hn "}

// minTitleKeyWords keeps short, generic titles ("v3.2.0", "Release notes",
// "Community meeting notes") out of title de-duplication, where unrelated
// articles would collide.
const minTitleKeyWords = 4

// TitleKey reduces a title to a comparison key for spotting the same story
// published under different URLs: lowercase, punctuation removed, HN prefixes
// stripped. It returns "" for titles too short to compare safely.
func TitleKey(title string) string {
	key := strings.TrimSpace(nonAlphanumeric.ReplaceAllString(strings.ToLower(title), " "))

	for _, prefix := range hnPrefixes {
		key = strings.TrimPrefix(key, prefix)
	}

	if len(strings.Fields(key)) < minTitleKeyWords {
		return ""
	}

	return key
}
