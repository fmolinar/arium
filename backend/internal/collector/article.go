package collector

import "time"

// Topic slugs shared with the frontend (arium/src/data/mockNews.js TOPICS).
const (
	TopicDevOps    = "devops"
	TopicSRE       = "sre"
	TopicGitOps    = "gitops"
	TopicDevSecOps = "devsecops"
)

// Article is the normalized shape every source is converted into. The JSON
// field names match the frontend's news items so the output can be served
// as-is later on.
type Article struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Source      string    `json:"source"`
	URL         string    `json:"url"`
	Tags        []string  `json:"tags"`
	PublishedAt time.Time `json:"publishedAt"`
	FetchedAt   time.Time `json:"fetchedAt"`
	// Origin is the name of the collector source that produced the article,
	// e.g. "kubernetes-blog" or "hackernews".
	Origin string `json:"origin"`
}
