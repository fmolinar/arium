package collector

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"
)

// ErrAllSourcesFailed is returned when every source in a run failed, so
// schedulers can alert on it. Partial failures are only recorded in the
// manifest.
var ErrAllSourcesFailed = errors.New("all sources failed")

// Collector fetches from its sources concurrently and turns the results into
// normalized, tagged articles.
type Collector struct {
	Sources []Source
	Tagger  *Tagger

	// SourceTimeout bounds each source's Fetch. Zero means no extra timeout.
	SourceTimeout time.Duration

	// MaxAge drops articles published longer ago than this. Zero keeps all.
	MaxAge time.Duration

	// Now defaults to time.Now; tests override it.
	Now func() time.Time
}

// Result is the in-memory outcome of Collect, before anything is stored.
type Result struct {
	Run      Run
	Sources  []SourceResult
	Raw      []Raw // parallel to Sources; empty Data for failed sources
	Articles []Article
}

// Collect runs every source and returns the combined result. Articles are
// de-duplicated by ID and sorted newest first. It returns
// ErrAllSourcesFailed (alongside the result) when no source succeeded.
func (c *Collector) Collect(ctx context.Context) (Result, error) {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}

	result := Result{
		Run:     NewRun(now()),
		Sources: make([]SourceResult, len(c.Sources)),
		Raw:     make([]Raw, len(c.Sources)),
	}

	fetched := make([][]Article, len(c.Sources))

	var wg sync.WaitGroup
	for i, src := range c.Sources {
		wg.Go(func() {
			result.Sources[i].Name = src.Name()
			result.Raw[i], fetched[i], result.Sources[i].Error = c.fetch(ctx, src)
		})
	}
	wg.Wait()

	fetchedAt := now()
	byID := map[string]bool{}
	failed := 0

	for i, sr := range result.Sources {
		if sr.Error != "" {
			failed++
			continue
		}

		result.Sources[i].Fetched = len(fetched[i])

		for _, item := range fetched[i] {
			item.Origin = sr.Name

			a, err := Normalize(item, fetchedAt)
			if err != nil {
				result.Sources[i].Rejected++
				continue
			}

			if c.MaxAge > 0 && fetchedAt.Sub(a.PublishedAt) > c.MaxAge {
				result.Sources[i].Stale++
				continue
			}

			if byID[a.ID] {
				continue
			}
			byID[a.ID] = true

			if c.Tagger != nil {
				a.Tags = c.Tagger.Tag(a)
			} else if a.Tags == nil {
				a.Tags = []string{}
			}

			result.Articles = append(result.Articles, a)
		}
	}

	slices.SortStableFunc(result.Articles, func(a, b Article) int {
		return b.PublishedAt.Compare(a.PublishedAt)
	})

	if len(c.Sources) > 0 && failed == len(c.Sources) {
		return result, ErrAllSourcesFailed
	}

	return result, nil
}

func (c *Collector) fetch(ctx context.Context, src Source) (raw Raw, articles []Article, errMsg string) {
	if c.SourceTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.SourceTimeout)
		defer cancel()
	}

	defer func() {
		if r := recover(); r != nil {
			raw, articles, errMsg = Raw{}, nil, fmt.Sprintf("panic: %v", r)
		}
	}()

	raw, articles, err := src.Fetch(ctx)
	if err != nil {
		return Raw{}, nil, err.Error()
	}

	return raw, articles, ""
}

// Persist writes a Result to the store while holding its lock: raw payloads
// (skipping unchanged ones), articles not already stored, then prunes data
// older than retention (0 disables pruning), and finally writes the manifest.
// A raw payload that fails to write is recorded as that source's error rather
// than aborting the run. It returns ErrLocked if another run holds the store.
func Persist(store *Store, result Result, now time.Time, retention time.Duration) (Manifest, error) {
	unlock, err := store.Lock()
	if err != nil {
		return Manifest{}, err
	}
	defer unlock()

	for i, raw := range result.Raw {
		if result.Sources[i].Error != "" || len(raw.Data) == 0 {
			continue
		}

		path, unchanged, err := store.WriteRaw(result.Run, result.Sources[i].Name, raw)
		if err != nil {
			result.Sources[i].Error = err.Error()
			continue
		}

		result.Sources[i].RawPath = path
		result.Sources[i].RawUnchanged = unchanged
	}

	written, err := store.WriteArticles(result.Run, result.Articles)
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		RunID:             result.Run.ID,
		StartedAt:         result.Run.StartedAt,
		Sources:           result.Sources,
		ArticlesWritten:   written.Written,
		ArticlesPath:      written.Path,
		DuplicatesByURL:   written.DuplicatesByURL,
		DuplicatesByTitle: written.DuplicatesByTitle,
	}

	if retention > 0 {
		pruned, err := store.Prune(now, retention)
		if err != nil {
			return Manifest{}, fmt.Errorf("prune: %w", err)
		}
		manifest.Pruned = &pruned
	}

	manifest.FinishedAt = now.UTC()

	return manifest, store.WriteManifest(manifest)
}
