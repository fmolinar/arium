package news

import "time"

// Topic slugs, shared with the collector and the frontend.
var Topics = []string{"devops", "sre", "gitops", "devsecops"}

// Article is a news item as stored in MongoDB and served by the API. The JSON
// field names match the collector's output and the frontend's news items.
type Article struct {
	ID          string    `bson:"_id"          json:"id"`
	Title       string    `bson:"title"        json:"title"`
	Summary     string    `bson:"summary"      json:"summary"`
	Source      string    `bson:"source"       json:"source"`
	URL         string    `bson:"url"          json:"url"`
	Tags        []string  `bson:"tags"         json:"tags"`
	PublishedAt time.Time `bson:"published_at" json:"publishedAt"`
	FetchedAt   time.Time `bson:"fetched_at"   json:"fetchedAt"`
	Origin      string    `bson:"origin"       json:"origin"`
}

// ListQuery selects a page of articles, newest first.
type ListQuery struct {
	Tag   string  // empty means every tag
	After *Cursor // continue after this article; nil starts at the newest
	Limit int
}

// Cursor is the position of the last article on a page. Articles are ordered
// by (PublishedAt, ID) descending so the order is stable when times tie.
type Cursor struct {
	PublishedAt time.Time
	ID          string
}

type ListResponse struct {
	Items []Article `json:"items"`
	// NextCursor fetches the following page; empty on the last page.
	NextCursor string `json:"nextCursor,omitempty"`
}

// ImportResult counts what Service.Import changed.
type ImportResult struct {
	Upserted int64
	Updated  int64
	Deleted  int64
}
