// Command collector fetches news from the configured sources and writes raw
// payloads, normalized articles and a run manifest to a directory (a Docker
// volume in deployment). See internal/collector for the on-disk layout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/collector/rss"
)

func main() {
	var (
		outDir  = flag.String("out", getEnv("COLLECTOR_OUT_DIR", "./data"), "output directory (env COLLECTOR_OUT_DIR)")
		sources = flag.String("sources", os.Getenv("COLLECTOR_SOURCES"), "sources config JSON; built-in list if empty (env COLLECTOR_SOURCES)")
		maxAge  = flag.Duration("since", getEnvDuration("COLLECTOR_SINCE", 7*24*time.Hour), "skip articles older than this; 0 keeps all (env COLLECTOR_SINCE)")
		timeout = flag.Duration("timeout", getEnvDuration("COLLECTOR_SOURCE_TIMEOUT", 15*time.Second), "per-source timeout (env COLLECTOR_SOURCE_TIMEOUT)")
		dryRun  = flag.Bool("dry-run", false, "print articles as NDJSON to stdout instead of writing to -out")
	)
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("collector: ")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadSources(*sources)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	var srcs []collector.Source
	for _, feed := range cfg.Feeds {
		srcs = append(srcs, rss.New(feed, client))
	}

	c := &collector.Collector{
		Sources:       srcs,
		Tagger:        collector.DefaultTagger(),
		SourceTimeout: *timeout,
		MaxAge:        *maxAge,
	}

	result, collectErr := c.Collect(ctx)

	for _, sr := range result.Sources {
		if sr.Error != "" {
			log.Printf("%-22s FAILED: %s", sr.Name, sr.Error)
			continue
		}
		log.Printf("%-22s fetched=%d rejected=%d stale=%d", sr.Name, sr.Fetched, sr.Rejected, sr.Stale)
	}

	if *dryRun {
		enc := json.NewEncoder(os.Stdout)
		for _, a := range result.Articles {
			if err := enc.Encode(a); err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("dry run: %d articles, nothing written", len(result.Articles))
	} else {
		store, err := collector.NewStore(*outDir)
		if err != nil {
			log.Fatal(err)
		}

		manifest, err := collector.Persist(store, result, time.Now())
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("run %s: %d new of %d articles → %s", manifest.RunID, manifest.ArticlesWritten, len(result.Articles), *outDir)
	}

	// Exit non-zero only when nothing could be fetched, so a scheduler alerts
	// on a full outage but not on one flaky feed.
	if errors.Is(collectErr, collector.ErrAllSourcesFailed) {
		log.Fatal(collectErr)
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	d, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("collector: %s: %v", key, err)
	}

	return d
}
