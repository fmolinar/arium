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
