// Command collector fetches news from the configured sources and writes raw
// payloads, normalized articles and a run manifest to a directory (a Docker
// volume in deployment). See internal/collector for the on-disk layout.
//
// By default it runs once and exits. With -schedule it stays up and runs at
// the given times every day, plus once at startup with -run-on-start. With -mongo-uri each run also syncs the stored
// articles into MongoDB for the API to serve. Logs are JSON on stdout, and with
// -metrics-address Prometheus metrics are served at /metrics.
//
// On AWS Lambda (AWS_LAMBDA_RUNTIME_API set) it serves one run per
// invocation and stores articles in MongoDB only; see lambda.go.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
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
	"github.com/fmolinar/arium/backend/internal/logging"
	"github.com/fmolinar/arium/backend/internal/news"
	"github.com/fmolinar/arium/backend/internal/telemetry"
)

type options struct {
	outDir     string
	dryRun     bool
	retention  time.Duration
	healthFile string
	news       *news.Service // nil when MongoDB sync is off
	// mongoOnly stores articles in news only, deduplicating against MongoDB
	// instead of the file store in outDir (Lambda mode).
	mongoOnly bool
	metrics   *runMetrics
}

func main() {
	logging.Setup("arium-collector")

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

		metricsAddr = flag.String("metrics-address", os.Getenv("COLLECTOR_METRICS_ADDRESS"), `serve Prometheus metrics at /metrics on this address, e.g. ":9464"; empty disables (env COLLECTOR_METRICS_ADDRESS)`)

		healthFile  = flag.String("health-file", getEnv("COLLECTOR_HEALTH_FILE", filepath.Join(os.TempDir(), "collector-heartbeat.json")), "heartbeat file written in schedule mode (env COLLECTOR_HEALTH_FILE)")
		healthcheck = flag.Bool("healthcheck", false, "exit non-zero if the scheduler's next run is overdue (for a container healthcheck)")
	)
	flag.Parse()

	if *healthcheck {
		if err := checkHeartbeat(*healthFile, time.Now()); err != nil {
			logging.Fatal("unhealthy", "error", err)
		}
		return
	}

	// Pruning drops dedup state older than -retention, so articles older than
	// that must already be filtered out by -since or they'd be stored again.
	if *retention > 0 && (*maxAge <= 0 || *maxAge > *retention) {
		logging.Fatal("-since must be set and no longer than -retention, or old articles would be re-stored after pruning",
			"since", maxAge.String(), "retention", retention.String())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := loadSources(*sources)
	if err != nil {
		logging.Fatal("load sources", "error", err)
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

	metrics, err := telemetry.NewMetrics("arium-collector")
	if err != nil {
		logging.Fatal("set up metrics", "error", err)
	}
	defer metrics.Shutdown(context.Background())

	if *metricsAddr != "" && !*dryRun {
		metrics.Serve(ctx, *metricsAddr)
	}

	runMetrics, err := newRunMetrics(metrics.Meter("github.com/fmolinar/arium/backend/cmd/collector"))
	if err != nil {
		logging.Fatal("set up metrics", "error", err)
	}

	opts := options{outDir: *outDir, dryRun: *dryRun, retention: *retention, healthFile: *healthFile, metrics: runMetrics}

	if lambdaRuntime() {
		opts.mongoOnly = true
		if err := startLambda(ctx, c, opts, *mongoURI, *mongoDB); err != nil {
			logging.Fatal("lambda", "error", err)
		}
		return
	}

	if *mongoURI != "" && !*dryRun {
		// Connect doesn't contact the server, so an unreachable MongoDB fails
		// individual syncs instead of stopping the collector.
		client, err := mongo.Connect(mongooptions.Client().ApplyURI(*mongoURI).SetServerSelectionTimeout(10 * time.Second))
		if err != nil {
			logging.Fatal("-mongo-uri", "error", err)
		}
		defer client.Disconnect(context.Background())

		opts.news = news.NewService(news.NewRepository(client.Database(*mongoDB)))
	}

	if *schedule == "" || *once {
		// Exit non-zero only when nothing could be fetched, so a scheduler
		// alerts on a full outage but not on one flaky feed.
		if err := run(ctx, c, opts); err != nil {
			logging.Fatal("run failed", "error", err)
		}
		return
	}

	if *dryRun {
		logging.Fatal("-dry-run can't be combined with -schedule; add -once")
	}

	loc, err := time.LoadLocation(*timezone)
	if err != nil {
		logging.Fatal("-timezone", "error", err)
	}

	sched, err := collector.ParseSchedule(*schedule, loc)
	if err != nil {
		logging.Fatal("-schedule", "error", err)
	}

	runScheduled(ctx, c, opts, sched, *onStart)
}

// runScheduled runs the collector at every scheduled time until ctx is
// cancelled, and first once right away if runNow is set. A failed run is
// logged and the schedule carries on. Runs missed while the process was down
// are not caught up.
func runScheduled(ctx context.Context, c *collector.Collector, opts options, sched collector.Schedule, runNow bool) {
	slog.Info("scheduled", "times", sched.String(), "retention", opts.retention.String())

	if runNow {
		// A heartbeat due now gives the startup run the same grace period as
		// a scheduled one before -healthcheck reports it as hung.
		if err := writeHeartbeat(opts.healthFile, time.Now()); err != nil {
			slog.Error("heartbeat", "error", err)
		}

		slog.Info("startup run")
		if err := run(ctx, c, opts); err != nil {
			slog.Error("run failed", "error", err)
		}
	}

	for {
		next := sched.Next(time.Now())
		slog.Info("next run", "at", next.Format(time.RFC3339))

		if err := writeHeartbeat(opts.healthFile, next); err != nil {
			slog.Error("heartbeat", "error", err)
		}

		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			slog.Info("shutting down")
			return
		case <-timer.C:
		}

		if err := run(ctx, c, opts); err != nil {
			slog.Error("run failed", "error", err)
		}
	}
}

// run is runOnce, recording the run's result and duration in opts.metrics.
func run(ctx context.Context, c *collector.Collector, opts options) error {
	start := time.Now()
	err := runOnce(ctx, c, opts)
	opts.metrics.recordRun(ctx, start, err)

	return err
}

// runOnce collects from every source and stores (or, in dry-run mode,
// prints) the result.
func runOnce(ctx context.Context, c *collector.Collector, opts options) error {
	result, collectErr := c.Collect(ctx)

	for _, sr := range result.Sources {
		if sr.Error != "" {
			slog.Warn("source failed", "source", sr.Name, "error", sr.Error)
			continue
		}
		slog.Info("source fetched", "source", sr.Name, "fetched", sr.Fetched, "rejected", sr.Rejected, "stale", sr.Stale)
	}
	opts.metrics.recordSources(ctx, result.Sources)

	if opts.dryRun {
		enc := json.NewEncoder(os.Stdout)
		for _, a := range result.Articles {
			if err := enc.Encode(a); err != nil {
				return err
			}
		}
		slog.Info("dry run, nothing written", "articles", len(result.Articles))

		return collectErr
	}

	if opts.mongoOnly {
		res, err := ingestNews(ctx, opts.news, result.Articles, time.Now(), opts.retention)
		if err != nil {
			return fmt.Errorf("store in MongoDB: %w", err)
		}
		opts.metrics.recordIngested(ctx, res)

		slog.Info("run stored", "run_id", result.Run.ID,
			"new", res.Inserted,
			"duplicates_by_url", res.DuplicatesByURL,
			"duplicates_by_title", res.DuplicatesByTitle,
			"expired", res.Deleted,
			"sources", len(result.Sources))

		if errors.Is(collectErr, collector.ErrAllSourcesFailed) {
			return collectErr
		}
		return nil
	}

	store, err := collector.NewStore(opts.outDir)
	if err != nil {
		return err
	}

	m, err := collector.Persist(store, result, time.Now(), opts.retention)
	if err != nil {
		return err
	}
	opts.metrics.recordStored(ctx, m)

	unchanged := 0
	for _, sr := range m.Sources {
		if sr.RawUnchanged {
			unchanged++
		}
	}

	attrs := []any{
		"run_id", m.RunID,
		"new", m.ArticlesWritten,
		"duplicates_by_url", m.DuplicatesByURL,
		"duplicates_by_title", m.DuplicatesByTitle,
		"raw_unchanged", unchanged,
		"sources", len(m.Sources),
	}
	if m.Pruned != nil {
		attrs = append(attrs, "pruned_files", m.Pruned.Files, "pruned_state_entries", m.Pruned.StateEntries)
	}
	slog.Info("run stored", attrs...)

	if opts.news != nil {
		res, err := syncNews(ctx, opts.news, store, time.Now(), opts.retention)
		if err != nil {
			return fmt.Errorf("sync to MongoDB: %w", err)
		}
		slog.Info("mongo sync", "inserted", res.Upserted, "updated", res.Updated, "expired", res.Deleted)
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
		logging.Fatal("invalid environment variable", "name", key, "error", err)
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
		logging.Fatal("invalid environment variable", "name", key, "error", err)
	}

	return d
}
