// Command collector fetches news from the configured sources and writes raw
// payloads, normalized articles and a run manifest to a directory (a Docker
// volume in deployment). See internal/collector for the on-disk layout.
//
// By default it runs once and exits. With -schedule it stays up and runs at
// the given times every day, plus once at startup with -run-on-start. With -mongo-uri each run also syncs the stored
// articles into MongoDB for the API to serve.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata" // -timezone works in minimal images without zoneinfo

	"go.mongodb.org/mongo-driver/v2/mongo"
	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/fmolinar/arium/backend/internal/collector"
	"github.com/fmolinar/arium/backend/internal/collector/hackernews"
	"github.com/fmolinar/arium/backend/internal/collector/rss"
	"github.com/fmolinar/arium/backend/internal/news"
)

type options struct {
	outDir     string
	dryRun     bool
	retention  time.Duration
	healthFile string
	news       *news.Service // nil when MongoDB sync is off
}

func main() {
	var (
		outDir    = flag.String("out", getEnv("COLLECTOR_OUT_DIR", "./data"), "output directory (env COLLECTOR_OUT_DIR)")
		sources   = flag.String("sources", os.Getenv("COLLECTOR_SOURCES"), "sources config JSON; built-in list if empty (env COLLECTOR_SOURCES)")
		maxAge    = flag.Duration("since", getEnvDuration("COLLECTOR_SINCE", 7*24*time.Hour), "skip articles older than this (env COLLECTOR_SINCE)")
		timeout   = flag.Duration("timeout", getEnvDuration("COLLECTOR_SOURCE_TIMEOUT", 15*time.Second), "per-source timeout (env COLLECTOR_SOURCE_TIMEOUT)")
		retention = flag.Duration("retention", getEnvDuration("COLLECTOR_RETENTION", 30*24*time.Hour), "delete stored data older than this; 0 keeps everything (env COLLECTOR_RETENTION)")
		schedule  = flag.String("schedule", os.Getenv("COLLECTOR_SCHEDULE"), `daily run times, e.g. "00:00,08:00,16:00"; empty runs once (env COLLECTOR_SCHEDULE)`)
		timezone  = flag.String("timezone", getEnv("COLLECTOR_TIMEZONE", "UTC"), "time zone for -schedule, e.g. America/Los_Angeles (env COLLECTOR_TIMEZONE)")
		once      = flag.Bool("once", false, "run once and exit, even if a schedule is configured")
		onStart   = flag.Bool("run-on-start", getEnvBool("COLLECTOR_RUN_ON_START", false), "with -schedule, also run once at startup instead of waiting for the first scheduled time (env COLLECTOR_RUN_ON_START)")
		dryRun    = flag.Bool("dry-run", false, "print articles as NDJSON to stdout instead of writing to -out")

		mongoURI = flag.String("mongo-uri", os.Getenv("MONGO_URI"), "sync stored articles into this MongoDB after each run; empty disables (env MONGO_URI)")
		mongoDB  = flag.String("mongo-db", getEnv("MONGO_DB", "arium"), "MongoDB database for -mongo-uri (env MONGO_DB)")

		healthFile  = flag.String("health-file", getEnv("COLLECTOR_HEALTH_FILE", filepath.Join(os.TempDir(), "collector-heartbeat.json")), "heartbeat file written in schedule mode (env COLLECTOR_HEALTH_FILE)")
		healthcheck = flag.Bool("healthcheck", false, "exit non-zero if the scheduler's next run is overdue (for a container healthcheck)")
	)
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("collector: ")

	if *healthcheck {
		if err := checkHeartbeat(*healthFile, time.Now()); err != nil {
			log.Fatalf("unhealthy: %v", err)
		}
		return
	}

	// Pruning drops dedup state older than -retention, so articles older than
	// that must already be filtered out by -since or they'd be stored again.
	if *retention > 0 && (*maxAge <= 0 || *maxAge > *retention) {
		log.Fatalf("-since (%s) must be set and no longer than -retention (%s), or old articles would be re-stored after pruning", *maxAge, *retention)
	}

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
	if cfg.HackerNews != nil {
		hn := *cfg.HackerNews
		hn.MaxAge = *maxAge
		srcs = append(srcs, hackernews.New(hn, client))
	}

	c := &collector.Collector{
		Sources:       srcs,
		Tagger:        collector.DefaultTagger(),
		SourceTimeout: *timeout,
		MaxAge:        *maxAge,
	}

	opts := options{outDir: *outDir, dryRun: *dryRun, retention: *retention, healthFile: *healthFile}

	if *mongoURI != "" && !*dryRun {
		// Connect doesn't contact the server, so an unreachable MongoDB fails
		// individual syncs instead of stopping the collector.
		client, err := mongo.Connect(mongooptions.Client().ApplyURI(*mongoURI).SetServerSelectionTimeout(10 * time.Second))
		if err != nil {
			log.Fatalf("-mongo-uri: %v", err)
		}
		defer client.Disconnect(context.Background())

		opts.news = news.NewService(news.NewRepository(client.Database(*mongoDB)))
	}

	if *schedule == "" || *once {
		// Exit non-zero only when nothing could be fetched, so a scheduler
		// alerts on a full outage but not on one flaky feed.
		if err := runOnce(ctx, c, opts); err != nil {
			log.Fatal(err)
		}
		return
	}

	if *dryRun {
		log.Fatal("-dry-run can't be combined with -schedule; add -once")
	}

	loc, err := time.LoadLocation(*timezone)
	if err != nil {
		log.Fatalf("-timezone: %v", err)
	}

	sched, err := collector.ParseSchedule(*schedule, loc)
	if err != nil {
		log.Fatalf("-schedule: %v", err)
	}

	runScheduled(ctx, c, opts, sched, *onStart)
}

// runScheduled runs the collector at every scheduled time until ctx is
// cancelled, and first once right away if runNow is set. A failed run is
// logged and the schedule carries on. Runs missed while the process was down
// are not caught up.
func runScheduled(ctx context.Context, c *collector.Collector, opts options, sched collector.Schedule, runNow bool) {
	log.Printf("scheduled: daily at %s, retention %s", sched, opts.retention)

	if runNow {
		// A heartbeat due now gives the startup run the same grace period as
		// a scheduled one before -healthcheck reports it as hung.
		if err := writeHeartbeat(opts.healthFile, time.Now()); err != nil {
			log.Printf("heartbeat: %v", err)
		}

		log.Print("startup run")
		if err := runOnce(ctx, c, opts); err != nil {
			log.Printf("run failed: %v", err)
		}
	}

	for {
		next := sched.Next(time.Now())
		log.Printf("next run at %s", next.Format(time.RFC3339))

		if err := writeHeartbeat(opts.healthFile, next); err != nil {
			log.Printf("heartbeat: %v", err)
		}

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Print("shutting down")
			return
		case <-timer.C:
		}

		if err := runOnce(ctx, c, opts); err != nil {
			log.Printf("run failed: %v", err)
		}
	}
}

// runOnce collects from every source and stores (or, in dry-run mode,
// prints) the result.
func runOnce(ctx context.Context, c *collector.Collector, opts options) error {
	result, collectErr := c.Collect(ctx)

	for _, sr := range result.Sources {
		if sr.Error != "" {
			log.Printf("%-22s FAILED: %s", sr.Name, sr.Error)
			continue
		}
		log.Printf("%-22s fetched=%d rejected=%d stale=%d", sr.Name, sr.Fetched, sr.Rejected, sr.Stale)
	}

	if opts.dryRun {
		enc := json.NewEncoder(os.Stdout)
		for _, a := range result.Articles {
			if err := enc.Encode(a); err != nil {
				return err
			}
		}
		log.Printf("dry run: %d articles, nothing written", len(result.Articles))

		return collectErr
	}

	store, err := collector.NewStore(opts.outDir)
	if err != nil {
		return err
	}

	m, err := collector.Persist(store, result, time.Now(), opts.retention)
	if err != nil {
		return err
	}

	unchanged := 0
	for _, sr := range m.Sources {
		if sr.RawUnchanged {
			unchanged++
		}
	}

	summary := fmt.Sprintf("run %s: %d new, %d duplicates (%d by URL, %d by title), %d/%d raw payloads unchanged",
		m.RunID, m.ArticlesWritten, m.DuplicatesByURL+m.DuplicatesByTitle, m.DuplicatesByURL, m.DuplicatesByTitle,
		unchanged, len(m.Sources))
	if m.Pruned != nil {
		summary += fmt.Sprintf(", pruned %d files and %d state entries", m.Pruned.Files, m.Pruned.StateEntries)
	}
	log.Print(summary)

	if opts.news != nil {
		res, err := syncNews(ctx, opts.news, store, time.Now(), opts.retention)
		if err != nil {
			return fmt.Errorf("sync to MongoDB: %w", err)
		}
		log.Printf("mongo: %d inserted, %d updated, %d expired", res.Upserted, res.Updated, res.Deleted)
	}

	if errors.Is(collectErr, collector.ErrAllSourcesFailed) {
		return collectErr
	}

	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	b, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("collector: %s: %v", key, err)
	}

	return b
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
