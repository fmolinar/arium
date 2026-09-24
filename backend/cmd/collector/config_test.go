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
	if len(cfg.Feeds) < 2 {
		t.Errorf("got %d default feeds, want at least 2", len(cfg.Feeds))
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

func TestLoadSourcesRejects(t *testing.T) {
	tests := map[string]string{
		"no feeds":       `{"feeds":[]}`,
		"bad json":       `{`,
		"unsafe name":    `{"feeds":[{"name":"../x","url":"https://a.test"}]}`,
		"duplicate name": `{"feeds":[{"name":"a","url":"https://a.test"},{"name":"a","url":"https://b.test"}]}`,
		"relative url":   `{"feeds":[{"name":"a","url":"/feed.xml"}]}`,
		"non-http url":   `{"feeds":[{"name":"a","url":"file:///etc/passwd"}]}`,
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
