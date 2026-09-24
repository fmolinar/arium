package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSourcesAreValid(t *testing.T) {
	cfg, err := loadSources("")
	if err != nil {
		t.Fatalf("built-in sources.json is invalid: %v", err)
	}
	if len(cfg.Feeds) == 0 || cfg.HackerNews == nil || len(cfg.HackerNews.Queries) == 0 {
		t.Errorf("defaults = %d feeds, hackernews %+v; want both source types configured", len(cfg.Feeds), cfg.HackerNews)
	}
}

func TestLoadSourcesFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.json")
	if err := os.WriteFile(path, []byte(`{"feeds":[{"name":"one","url":"https://a.test/feed"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadSources(path)
	if err != nil {
		t.Fatalf("loadSources: %v", err)
	}
	if len(cfg.Feeds) != 1 || cfg.Feeds[0].Name != "one" {
		t.Errorf("feeds = %+v", cfg.Feeds)
	}
}

func TestLoadSourcesHackerNewsOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.json")
	if err := os.WriteFile(path, []byte(`{"hackernews":{"queries":[{"query":"kubernetes"}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadSources(path)
	if err != nil {
		t.Fatalf("loadSources: %v", err)
	}
	if len(cfg.Feeds) != 0 || cfg.HackerNews == nil {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoadSourcesRejects(t *testing.T) {
	tests := map[string]string{
		"no feeds":       `{"feeds":[]}`,
		"bad json":       `{`,
		"unsafe name":    `{"feeds":[{"name":"../x","url":"https://a.test"}]}`,
		"duplicate name": `{"feeds":[{"name":"a","url":"https://a.test"},{"name":"a","url":"https://b.test"}]}`,
		"relative url":   `{"feeds":[{"name":"a","url":"/feed.xml"}]}`,
		"non-http url":   `{"feeds":[{"name":"a","url":"file:///etc/passwd"}]}`,
		"hn no queries":  `{"hackernews":{"queries":[]}}`,
		"hn empty query": `{"hackernews":{"queries":[{"query":"  "}]}}`,
		"hn dup query":   `{"hackernews":{"queries":[{"query":"k8s"},{"query":"K8s "}]}}`,
		"hn negative":    `{"hackernews":{"minPoints":-1,"queries":[{"query":"k8s"}]}}`,
		"name clash":     `{"feeds":[{"name":"hackernews","url":"https://a.test"}],"hackernews":{"queries":[{"query":"k8s"}]}}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sources.json")
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}

			if _, err := loadSources(path); err == nil {
				t.Error("loadSources succeeded, want error")
			}
		})
	}

	if _, err := loadSources(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "read sources config") {
		t.Errorf("missing file error = %v", err)
	}
}
