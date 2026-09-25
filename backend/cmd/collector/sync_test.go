package main

import (
	"testing"
	"time"

	"github.com/fmolinar/arium/backend/internal/collector"
)

func TestImportable(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-30 * 24 * time.Hour)

	stored := []collector.Article{
		{ID: "a", Title: "first copy", FetchedAt: now},
		{ID: "old", Title: "expired", FetchedAt: cutoff.Add(-time.Second)},
		{ID: "b", Title: "b", FetchedAt: cutoff, Tags: []string{"sre"}},
		{ID: "a", Title: "second copy", FetchedAt: now},
	}

	got := importable(stored, cutoff)

	if len(got) != 2 {
		t.Fatalf("importable = %+v, want a and b", got)
	}
	if got[0].ID != "a" || got[0].Title != "second copy" {
		t.Errorf("got[0] = %+v, want the last copy of a", got[0])
	}
	if got[0].Tags == nil {
		t.Error("nil tags must become an empty slice so the API returns [] not null")
	}
	if got[1].ID != "b" || len(got[1].Tags) != 1 {
		t.Errorf("got[1] = %+v, want b with its tag", got[1])
	}
}
