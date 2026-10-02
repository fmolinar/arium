//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/fmolinar/arium/backend/internal/news"
)

type newsPage struct {
	Items      []news.Article `json:"items"`
	NextCursor string         `json:"nextCursor"`
}

func TestNewsImportAndList(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Five articles, newest first; a and b share a publish time to exercise
	// the cursor's tie-breaking on ID.
	articles := []news.Article{
		{ID: "news-e", Tags: []string{"sre"}, PublishedAt: now.Add(-1 * time.Hour)},
		{ID: "news-d", Tags: []string{"devops", "sre"}, PublishedAt: now.Add(-2 * time.Hour)},
		{ID: "news-c", Tags: []string{"gitops"}, PublishedAt: now.Add(-3 * time.Hour)},
		{ID: "news-b", Tags: []string{"sre"}, PublishedAt: now.Add(-4 * time.Hour)},
		{ID: "news-a", Tags: []string{"devsecops"}, PublishedAt: now.Add(-4 * time.Hour)},
	}
	for i := range articles {
		articles[i].Title = "Title " + articles[i].ID
		articles[i].FetchedAt = now
	}
	// Fetched outside the retention window: Import must delete it.
	expired := news.Article{ID: "news-old", Tags: []string{"sre"}, PublishedAt: now.Add(-40 * 24 * time.Hour), FetchedAt: now.Add(-31 * 24 * time.Hour)}

	if _, err := newsService.Import(ctx, append([]news.Article{expired}, articles...), now, 0); err != nil {
		t.Fatalf("seed import: %v", err)
	}

	// Importing again is idempotent, and applies retention.
	res, err := newsService.Import(ctx, articles, now, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Upserted != 0 || res.Deleted != 1 {
		t.Errorf("re-import = %+v, want 0 upserted and 1 deleted", res)
	}

	t.Run("pages through everything in order", func(t *testing.T) {
		var ids []string
		cursor := ""
		for page := 0; page < 10; page++ {
			resp := request(t, http.MethodGet, "/api/v1/news?limit=2&cursor="+cursor, "", nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			p := decodeJSON[newsPage](t, resp)
			for _, a := range p.Items {
				ids = append(ids, a.ID)
			}
			if cursor = p.NextCursor; cursor == "" {
				break
			}
		}

		want := "[news-e news-d news-c news-b news-a]"
		if got := fmt.Sprint(ids); got != want {
			t.Errorf("ids = %s, want %s", got, want)
		}
	})

	t.Run("filters by tag", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/api/v1/news?tag=sre", "", nil)
		p := decodeJSON[newsPage](t, resp)

		var ids []string
		for _, a := range p.Items {
			ids = append(ids, a.ID)
		}
		if got, want := fmt.Sprint(ids), "[news-e news-d news-b]"; got != want {
			t.Errorf("ids = %s, want %s", got, want)
		}
		if p.NextCursor != "" {
			t.Errorf("nextCursor = %q on the last page, want empty", p.NextCursor)
		}
	})

	t.Run("rejects an unknown tag", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/api/v1/news?tag=frontend", "", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestNewsIngest(t *testing.T) {
	ctx := context.Background()
	first := time.Now().UTC().Truncate(time.Millisecond)
	retention := 30 * 24 * time.Hour

	article := func(id, titleKey string, fetchedAt time.Time) news.Article {
		return news.Article{ID: id, Title: id, TitleKey: titleKey, Tags: []string{"sre"}, PublishedAt: fetchedAt, FetchedAt: fetchedAt}
	}

	res, err := ingestService.Ingest(ctx, []news.Article{
		article("ingest-a", "same story", first),
		article("ingest-b", "", first),
		// Same story as ingest-a under another URL, in the same batch.
		article("ingest-c", "same story", first),
		// Same URL twice in the batch.
		article("ingest-b", "", first),
		article("ingest-old", "", first.Add(-31*24*time.Hour)),
	}, first, 0)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if want := (news.IngestResult{Inserted: 3, DuplicatesByURL: 1, DuplicatesByTitle: 1}); res != want {
		t.Errorf("first ingest = %+v, want %+v", res, want)
	}

	// A later run sees the same articles again, plus the story under a new
	// URL. Nothing is overwritten, so fetched_at keeps the first-seen time,
	// and retention deletes the expired article.
	later := first.Add(8 * time.Hour)
	res, err = ingestService.Ingest(ctx, []news.Article{
		article("ingest-a", "same story", later),
		article("ingest-b", "", later),
		article("ingest-d", "same story", later),
		article("ingest-e", "", later),
	}, later, retention)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if want := (news.IngestResult{Inserted: 1, DuplicatesByURL: 2, DuplicatesByTitle: 1, Deleted: 1}); res != want {
		t.Errorf("second ingest = %+v, want %+v", res, want)
	}

	page, err := ingestService.List(ctx, news.ListQuery{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	fetched := map[string]time.Time{}
	for _, a := range page.Items {
		fetched[a.ID] = a.FetchedAt.UTC()
	}
	if got, want := len(fetched), 3; got != want {
		t.Errorf("stored %d articles (%v), want %d", got, fetched, want)
	}
	if !fetched["ingest-a"].Equal(first) {
		t.Errorf("ingest-a fetched_at = %v, want first-seen %v", fetched["ingest-a"], first)
	}
	if !fetched["ingest-e"].Equal(later) {
		t.Errorf("ingest-e fetched_at = %v, want %v", fetched["ingest-e"], later)
	}
}
