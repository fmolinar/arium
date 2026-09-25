package news

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns one page of articles and the cursor for the next page.
func (s *Service) List(ctx context.Context, q ListQuery) (*ListResponse, error) {
	// Fetch one extra article to know whether another page exists.
	limit := q.Limit
	q.Limit++

	articles, err := s.repo.List(ctx, q)
	if err != nil {
		return nil, err
	}

	resp := &ListResponse{Items: articles}
	if len(articles) > limit {
		resp.Items = articles[:limit]
		last := resp.Items[limit-1]
		resp.NextCursor = EncodeCursor(Cursor{PublishedAt: last.PublishedAt, ID: last.ID})
	}

	return resp, nil
}

// Import upserts articles and, when retention is positive, deletes articles
// fetched longer ago than retention.
func (s *Service) Import(ctx context.Context, articles []Article, now time.Time, retention time.Duration) (ImportResult, error) {
	var result ImportResult

	var err error
	if result.Upserted, result.Updated, err = s.repo.Upsert(ctx, articles); err != nil {
		return result, err
	}

	if retention > 0 {
		if result.Deleted, err = s.repo.DeleteFetchedBefore(ctx, now.Add(-retention)); err != nil {
			return result, err
		}
	}

	return result, nil
}

// EncodeCursor returns an opaque, URL-safe token for c.
func EncodeCursor(c Cursor) string {
	raw := c.PublishedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID

	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor parses a token produced by EncodeCursor.
func DecodeCursor(token string) (Cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return Cursor{}, ErrInvalidCursor
	}

	publishedAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	return Cursor{PublishedAt: publishedAt, ID: id}, nil
}
