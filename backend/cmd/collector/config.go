package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/collector/hackernews"
	"github.com/fmolinar/arium/backend/internal/collector/rss"
)

// defaultSources is used when no -sources file is given.
//
//go:embed sources.json
var defaultSources []byte

type sourcesConfig struct {
	Feeds      []rss.Feed         `json:"feeds"`
	HackerNews *hackernews.Config `json:"hackernews,omitempty"`
}

// loadSources reads the sources config from path, or the built-in default
// when path is empty, and validates it.
func loadSources(path string) (sourcesConfig, error) {
	data := defaultSources

	if path != "" {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			return sourcesConfig{}, fmt.Errorf("read sources config: %w", err)
		}
	}

	var cfg sourcesConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return sourcesConfig{}, fmt.Errorf("decode sources config: %w", err)
	}

	return cfg, cfg.validate()
}

func (c sourcesConfig) validate() error {
	if len(c.Feeds) == 0 && c.HackerNews == nil {
		return errors.New("sources config has no feeds and no hackernews section")
	}

	// Feed names share a namespace with the other sources' fixed names.
	names := map[string]bool{}
	if c.HackerNews != nil {
		names[hackernews.Name] = true

		if err := validateHackerNews(*c.HackerNews); err != nil {
			return err
		}
	}

	for _, f := range c.Feeds {
		if !collector.ValidName(f.Name) {
			return fmt.Errorf("feed name %q must be letters, digits, '-' or '_'", f.Name)
		}
		if names[f.Name] {
			return fmt.Errorf("duplicate feed name %q", f.Name)
		}
		names[f.Name] = true

		u, err := url.Parse(f.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("feed %q: URL %q must be an absolute http(s) URL", f.Name, f.URL)
		}
	}

	return nil
}

func validateHackerNews(cfg hackernews.Config) error {
	if len(cfg.Queries) == 0 {
		return errors.New("hackernews: no queries")
	}
	if cfg.MinPoints < 0 || cfg.HitsPerQuery < 0 {
		return errors.New("hackernews: minPoints and hitsPerQuery must not be negative")
	}

	seen := map[string]bool{}
	for _, q := range cfg.Queries {
		key := strings.ToLower(strings.TrimSpace(q.Query))
		if key == "" {
			return errors.New("hackernews: empty query")
		}
		if seen[key] {
			return fmt.Errorf("hackernews: duplicate query %q", q.Query)
		}
		seen[key] = true
	}

	return nil
}
