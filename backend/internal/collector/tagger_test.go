package collector

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestDefaultTagger(t *testing.T) {
	tagger := DefaultTagger()

	tests := []struct {
		name    string
		article Article
		want    []string
	}{
		{
			name:    "title keyword",
			article: Article{Title: "Kubernetes 1.35 ships in-place resize"},
			want:    []string{TopicDevOps},
		},
		{
			name:    "summary keyword, case-insensitive",
			article: Article{Title: "Release notes", Summary: "Now with native OPENTELEMETRY export"},
			want:    []string{TopicSRE},
		},
		{
			name:    "multiple topics, sorted",
			article: Article{Title: "Argo CD patches CVE-2026-1234 in its Helm integration"},
			want:    []string{TopicDevOps, TopicDevSecOps, TopicGitOps},
		},
		{
			name:    "keeps and merges source tags",
			article: Article{Title: "Error budgets explained", Tags: []string{TopicSRE, "custom"}},
			want:    []string{"custom", TopicSRE},
		},
		{
			name:    "word boundaries: no match inside other words",
			article: Article{Title: "Sreekanth's dockerless helmet review"},
			want:    []string{},
		},
		{
			name:    "plain flux is not gitops",
			article: Article{Title: "Pricing is in flux this quarter"},
			want:    []string{},
		},
		{
			name:    "flux cd is gitops",
			article: Article{Title: "FluxCD 2.5 released"},
			want:    []string{TopicGitOps},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tagger.Tag(tt.article)
			if !slices.Equal(got, tt.want) {
				t.Errorf("Tag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTagNeverNil(t *testing.T) {
	tags := DefaultTagger().Tag(Article{Title: "nothing relevant"})

	data, _ := json.Marshal(tags)
	if string(data) != "[]" {
		t.Errorf("Tag() JSON = %s, want []", data)
	}
}

func TestTagDoesNotMutateInput(t *testing.T) {
	a := Article{Title: "Kubernetes", Tags: []string{"z", "a"}}

	DefaultTagger().Tag(a)

	if !slices.Equal(a.Tags, []string{"z", "a"}) {
		t.Errorf("input tags mutated: %v", a.Tags)
	}
}
