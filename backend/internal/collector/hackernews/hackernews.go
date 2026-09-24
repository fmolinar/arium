// Package hackernews is a collector source for Hacker News stories, searched
// by keyword through the Algolia HN Search API (no API key needed).
package hackernews

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/fmolinar/arium/backend/internal/collector"
)

const (
	// Name is the source name used in storage paths and manifests.
	Name = "hackernews"

	defaultBaseURL   = "https://hn.algolia.com/api/v1"
	defaultMinPoints = 20
	defaultHits      = 30
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

// Fetch runs the queries in order. Any failed query fails the whole source so
// the manifest shows it; the stored raw payload maps each query to its
// response body. A story matched by several queries is returned once, with
// the tags of all of them.
func (s *Source) Fetch(ctx context.Context) (collector.Raw, []collector.Article, error) {
	responses := make(map[string]json.RawMessage, len(s.cfg.Queries))
	var articles []collector.Article
	index := map[string]int{} // objectID → position in articles

	for _, q := range s.cfg.Queries {
		body, err := collector.HTTPGet(ctx, s.client, s.searchURL(q.Query), "application/json")
		if err != nil {
			return collector.Raw{}, nil, fmt.Errorf("query %q: %w", q.Query, err)
		}

		var resp searchResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return collector.Raw{}, nil, fmt.Errorf("query %q: decode response: %w", q.Query, err)
		}

		responses[q.Query] = body

		for _, h := range resp.Hits {
			if i, ok := index[h.ObjectID]; ok {
				articles[i].Tags = append(articles[i].Tags, q.Tags...)
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
