// Package rss is a collector source for RSS and Atom feeds.
package rss

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/mmcdole/gofeed"

	"github.com/fmolinar/arium/backend/internal/collector"
)

// Feed configures one feed.
type Feed struct {
	// Name is the source name used in storage paths, e.g. "kubernetes-blog".
	Name string `json:"name"`
	URL  string `json:"url"`
	// Tags are applied to every article from the feed, on top of keyword
	// tagging. Useful for single-topic feeds.
	Tags []string `json:"tags,omitempty"`
}

// Source fetches and parses one feed.
type Source struct {
	feed   Feed
	client *http.Client
}

// New returns a Source for feed. client must not be nil.
func New(feed Feed, client *http.Client) *Source {
	return &Source{feed: feed, client: client}
}

func (s *Source) Name() string {
	return s.feed.Name
}

func (s *Source) Fetch(ctx context.Context) (collector.Raw, []collector.Article, error) {
	body, err := collector.HTTPGet(ctx, s.client, s.feed.URL,
		"application/rss+xml, application/atom+xml, application/xml;q=0.9, text/xml;q=0.8")
	if err != nil {
		return collector.Raw{}, nil, err
	}

	parsed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return collector.Raw{}, nil, fmt.Errorf("parse feed %s: %w", s.feed.URL, err)
	}

	articles := make([]collector.Article, 0, len(parsed.Items))

	for _, item := range parsed.Items {
		a := collector.Article{
			Title:   item.Title,
			Summary: item.Description,
			URL:     item.Link,
			Tags:    append([]string{}, s.feed.Tags...),
		}

		if a.Summary == "" {
			a.Summary = item.Content
		}

		switch {
		case item.PublishedParsed != nil:
			a.PublishedAt = *item.PublishedParsed
		case item.UpdatedParsed != nil:
			a.PublishedAt = *item.UpdatedParsed
		}

		articles = append(articles, a)
	}

	return collector.Raw{Data: body, Ext: "xml"}, articles, nil
}
