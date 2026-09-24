package collector

import (
	"regexp"
	"slices"
	"strings"
)

// Tagger assigns topic tags from keyword matches in an article's text.
type Tagger struct {
	rules map[string]*regexp.Regexp
}

// DefaultTagger returns a Tagger with keyword rules for the four site topics.
func DefaultTagger() *Tagger {
	return NewTagger(map[string][]string{
		TopicDevOps: {
			`devops`, `kubernetes`, `k8s`, `docker`, `containers?`, `helm`,
			`terraform`, `opentofu`, `ansible`, `ci/cd`, `jenkins`,
			`github actions`, `infrastructure as code`, `platform engineering`,
		},
		TopicSRE: {
			`sre`, `site reliability`, `observability`, `prometheus`, `grafana`,
			`opentelemetry`, `slos?`, `slis?`, `error budgets?`, `incidents?`,
			`postmortems?`, `on-call`, `outages?`, `alerting`,
		},
		TopicGitOps: {
			// Plain "flux" is left out: "in flux" is too common in prose.
			`gitops`, `argo ?cd`, `flux ?cd`, `flux2`, `kustomize`,
		},
		TopicDevSecOps: {
			`devsecops`, `cve-\d{4}-\d+`, `vulnerabilit(y|ies)`, `supply[- ]chain`,
			`sbom`, `sigstore`, `cosign`, `exploit(ed|s)?`, `cisa`, `zero-day`,
			`ransomware`, `security advisory`,
		},
	})
}

// NewTagger builds a Tagger from topic → keyword patterns. Patterns are
// regular expressions matched case-insensitively on word boundaries.
func NewTagger(keywords map[string][]string) *Tagger {
	rules := make(map[string]*regexp.Regexp, len(keywords))

	for topic, patterns := range keywords {
		rules[topic] = regexp.MustCompile(`(?i)\b(?:` + strings.Join(patterns, "|") + `)\b`)
	}

	return &Tagger{rules: rules}
}

// Tag returns a sorted, de-duplicated list of the article's existing tags plus
// every topic whose keywords appear in its title or summary. It never returns
// nil, so the JSON output is always an array.
func (t *Tagger) Tag(a Article) []string {
	text := a.Title + "\n" + a.Summary
	tags := append([]string{}, a.Tags...)

	for topic, rule := range t.rules {
		if rule.MatchString(text) {
			tags = append(tags, topic)
		}
	}

	slices.Sort(tags)

	return slices.Compact(tags)
}
