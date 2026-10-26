// Package hackernews is a collector source for Hacker News stories, searched
// by keyword through the Algolia HN Search API (no API key needed).
package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Fetch runs the queries concurrently, at most maxConcurrent at a time. Any
// failed query fails the whole source so the manifest shows it; the stored
// raw payload maps each query to its response body. A story matched by
// several queries is returned once, with the tags of all of them, and the
// merge follows query order so the result doesn't depend on timing.
func (s *Source) Fetch(ctx context.Context) (collector.Raw, []collector.Article, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

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

			if bodies[i], hits[i], errs[i] = s.search(ctx, q.Query); errs[i] != nil {
				cancel() // the source fails anyway, so stop the other queries
			}
		}()
	}
	wg.Wait()

	// Report the first query that failed on its own, not one canceled
	// because of it.
	var firstErr error
	for _, err := range errs {
		if err != nil && (firstErr == nil || errors.Is(firstErr, context.Canceled) && !errors.Is(err, context.Canceled)) {
			firstErr = err
		}
	}
	if firstErr != nil {
		return collector.Raw{}, nil, firstErr
	}

	responses := make(map[string]json.RawMessage, len(s.cfg.Queries))
	var articles []collector.Article
	index := map[string]int{} // objectID → position in articles

	for i, q := range s.cfg.Queries {
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
