// Package hackernews is a collector source for Hacker News stories, searched
// by keyword through the Algolia HN Search API (no API key needed).
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/fmolinar/arium/backend/internal/collector"
)

const (
	// Name is the source name used in storage paths and manifests.
	Name = "hackernews"

	defaultBaseURL   = "https://hn.algolia.com/api/v1"
	defaultMinPoints = 20
	defaultHits      = 30

	// maxConcurrent bounds the queries in flight at once. Sequential queries
	// didn't fit in the per-source timeout from Lambda, where each Algolia
	// round trip is much slower than from a desktop.
	maxConcurrent = 4

	// A query gets maxAttempts tries of at most QueryTimeout each, so one hung
	// connection costs a retry instead of the whole source's timeout.
	maxAttempts         = 2
	defaultQueryTimeout = 20 * time.Second
	defaultRetryDelay   = time.Second
)

// Query is one keyword search. Tags are applied to every story it returns,
// on top of keyword tagging.
type Query struct {
	Query string   `json:"query"`
	Tags  []string `json:"tags,omitempty"`
}

// Config configures the source.
type Config struct {
	Queries []Query `json:"queries"`
	// MinPoints filters out stories with fewer upvotes. Defaults to 20.
	MinPoints int `json:"minPoints,omitempty"`
	// HitsPerQuery is how many of the newest matching stories each query
	// returns. Defaults to 30.
	HitsPerQuery int `json:"hitsPerQuery,omitempty"`
	// MaxAge, when set, asks the API only for stories newer than this, so old
	// matches aren't downloaded just to be dropped as stale. The collector CLI
	// sets it from -since.
	MaxAge time.Duration `json:"-"`
	// QueryTimeout bounds each attempt at a query. Defaults to 20s.
	QueryTimeout time.Duration `json:"-"`
	// RetryDelay is the wait before retrying a failed query. Defaults to 1s.
	RetryDelay time.Duration `json:"-"`
	// BaseURL overrides the Algolia API root; used in tests.
	BaseURL string `json:"-"`
	// Now defaults to time.Now; used in tests.
	Now func() time.Time `json:"-"`
}

// Source runs every configured query and merges the stories.
type Source struct {
	cfg    Config
	client *http.Client
}

// New returns a Source for cfg, filling in defaults. client must not be nil.
func New(cfg Config, client *http.Client) *Source {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.MinPoints == 0 {
		cfg.MinPoints = defaultMinPoints
	}
	if cfg.HitsPerQuery == 0 {
		cfg.HitsPerQuery = defaultHits
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = defaultQueryTimeout
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = defaultRetryDelay
	}

	return &Source{cfg: cfg, client: client}
}

func (s *Source) Name() string {
	return Name
}

type searchResponse struct {
	Hits []hit `json:"hits"`
}

type hit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	CreatedAtI  int64  `json:"created_at_i"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	StoryText   string `json:"story_text"`
}

// Fetch runs the queries concurrently, at most maxConcurrent at a time. A
// query that still fails after its retry is logged and left out, so a slow
// query (or the source timeout cutting off the last ones) doesn't discard the
// others' stories; the source fails only when every query does. The stored
// raw payload maps each successful query to its response body. A story
// matched by several queries is returned once, with the tags of all of them,
// and the merge follows query order so the result doesn't depend on timing.
func (s *Source) Fetch(ctx context.Context) (collector.Raw, []collector.Article, error) {
	bodies := make([]json.RawMessage, len(s.cfg.Queries))
	hits := make([][]hit, len(s.cfg.Queries))
	errs := make([]error, len(s.cfg.Queries))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)
	for i, q := range s.cfg.Queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			bodies[i], hits[i], errs[i] = s.searchWithRetry(ctx, q.Query)
		}()
	}
	wg.Wait()

	var failed []error
	for i, err := range errs {
		if err != nil {
			failed = append(failed, err)
			slog.WarnContext(ctx, "hackernews query failed", "query", s.cfg.Queries[i].Query, "error", err)
		}
	}
	if len(failed) > 0 && len(failed) == len(s.cfg.Queries) {
		return collector.Raw{}, nil, fmt.Errorf("all %d queries failed: %w", len(failed), failed[0])
	}

	responses := make(map[string]json.RawMessage, len(s.cfg.Queries))
	var articles []collector.Article
	index := map[string]int{} // objectID → position in articles

	for i, q := range s.cfg.Queries {
		if errs[i] != nil {
			continue
		}
		responses[q.Query] = bodies[i]

		for _, h := range hits[i] {
			if j, ok := index[h.ObjectID]; ok {
				articles[j].Tags = append(articles[j].Tags, q.Tags...)
				continue
			}

			index[h.ObjectID] = len(articles)
			articles = append(articles, toArticle(h, q.Tags))
		}
	}

	raw, err := json.Marshal(responses)
	if err != nil {
		return collector.Raw{}, nil, fmt.Errorf("encode raw payload: %w", err)
	}

	return collector.Raw{Data: raw, Ext: "json"}, articles, nil
}

// searchWithRetry runs one query, retrying it once if an attempt fails while
// ctx is still live, and logs how long each attempt took.
func (s *Source) searchWithRetry(ctx context.Context, query string) (json.RawMessage, []hit, error) {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, nil, err
			case <-time.After(s.cfg.RetryDelay):
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, s.cfg.QueryTimeout)
		start := time.Now()
		var body json.RawMessage
		var hits []hit
		body, hits, err = s.search(attemptCtx, query)
		cancel()

		attrs := []any{"query", query, "attempt", attempt, "duration_ms", time.Since(start).Milliseconds()}
		if err == nil {
			slog.InfoContext(ctx, "hackernews query", append(attrs, "hits", len(hits))...)
			return body, hits, nil
		}
		slog.InfoContext(ctx, "hackernews query attempt failed", append(attrs, "error", err)...)

		if ctx.Err() != nil {
			break // the source's own deadline, not this attempt's; don't retry
		}
	}

	return nil, nil, err
}

// search runs one query and returns the response body and its hits.
func (s *Source) search(ctx context.Context, query string) (json.RawMessage, []hit, error) {
	body, err := collector.HTTPGet(ctx, s.client, s.searchURL(query), "application/json")
	if err != nil {
		return nil, nil, fmt.Errorf("query %q: %w", query, err)
	}

	var resp searchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("query %q: decode response: %w", query, err)
	}

	return body, resp.Hits, nil
}

func (s *Source) searchURL(query string) string {
	params := url.Values{}
	params.Set("query", query)
	params.Set("tags", "story")
	params.Set("restrictSearchableAttributes", "title")
	filters := "points>=" + strconv.Itoa(s.cfg.MinPoints)
	if s.cfg.MaxAge > 0 {
		filters += ",created_at_i>" + strconv.FormatInt(s.cfg.Now().Add(-s.cfg.MaxAge).Unix(), 10)
	}
	params.Set("numericFilters", filters)
	params.Set("hitsPerPage", strconv.Itoa(s.cfg.HitsPerQuery))

	return s.cfg.BaseURL + "/search_by_date?" + params.Encode()
}

func discussionURL(objectID string) string {
	return "https://news.ycombinator.com/item?id=" + url.QueryEscape(objectID)
}

func toArticle(h hit, tags []string) collector.Article {
	a := collector.Article{
		Title:   h.Title,
		URL:     h.URL,
		Summary: h.StoryText,
		Tags:    append([]string{}, tags...),
	}

	if h.CreatedAtI > 0 {
		a.PublishedAt = time.Unix(h.CreatedAtI, 0)
	}

	// Ask/Show HN posts without a link point at the discussion itself.
	if a.URL == "" {
		a.URL = discussionURL(h.ObjectID)
	}

	if a.Summary == "" {
		a.Summary = fmt.Sprintf("Discussed on Hacker News: %d points, %d comments.", h.Points, h.NumComments)
	}

	return a
}
