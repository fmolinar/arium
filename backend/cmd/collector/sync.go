package main

import (
	"context"
	"fmt"
	"time"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/news"
)

// syncTimeout bounds one MongoDB sync, so an unreachable database fails the
// run instead of hanging it.
const syncTimeout = time.Minute

// syncNews loads every article still inside the retention window from the
// store into MongoDB and deletes older ones there. Syncing everything each
// run, not just this run's new articles, means a run that couldn't reach
// MongoDB is caught up by the next one.
func syncNews(ctx context.Context, svc *news.Service, store *collector.Store, now time.Time, retention time.Duration) (news.ImportResult, error) {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	stored, err := store.ReadArticles()
	if err != nil {
		return news.ImportResult{}, fmt.Errorf("read stored articles: %w", err)
	}

	var cutoff time.Time
	if retention > 0 {
		cutoff = now.Add(-retention)
	}

	return svc.Import(ctx, importable(stored, cutoff), now, retention)
}

// importable converts stored articles for MongoDB, dropping those fetched
// before cutoff and keeping only the last copy of any repeated ID.
func importable(stored []collector.Article, cutoff time.Time) []news.Article {
	index := map[string]int{}
	var out []news.Article

	for _, a := range stored {
		if a.FetchedAt.Before(cutoff) {
			continue
		}

		n := news.Article{
			ID:          a.ID,
			Title:       a.Title,
			Summary:     a.Summary,
			Source:      a.Source,
			URL:         a.URL,
			Tags:        a.Tags,
			PublishedAt: a.PublishedAt,
			FetchedAt:   a.FetchedAt,
			Origin:      a.Origin,
		}
		if n.Tags == nil {
			n.Tags = []string{}
		}

		if i, ok := index[a.ID]; ok {
			out[i] = n
			continue
		}
		index[a.ID] = len(out)
		out = append(out, n)
	}

	return out
}
