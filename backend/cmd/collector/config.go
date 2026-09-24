package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/collector/rss"
)

// defaultSources is used when no -sources file is given.
//
//go:embed sources.json
var defaultSources []byte

type sourcesConfig struct {
	Feeds []rss.Feed `json:"feeds"`
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
	if len(c.Feeds) == 0 {
		return errors.New("sources config has no feeds")
	}

	names := map[string]bool{}

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
