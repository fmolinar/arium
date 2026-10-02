package main

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/fmolinar/arium/backend/internal/collector"
)

// runMetrics are the collector's instruments. Prometheus only sees them while
// the process is up, so they're useful in schedule mode; a one-off run exits
// before it could be scraped.
type runMetrics struct {
	runs        metric.Int64Counter
	runDuration metric.Float64Histogram
	written     metric.Int64Counter
	duplicates  metric.Int64Counter
	fetched     metric.Int64Counter
	failures    metric.Int64Counter

	lastSuccess atomic.Int64 // unix seconds; 0 until a run succeeds
	lastWritten atomic.Int64 // articles stored by the last run; -1 until one stores
}

// runBuckets fit runs that take seconds to a few minutes.
var runBuckets = []float64{1, 2.5, 5, 10, 15, 30, 60, 120, 300, 600}

func newRunMetrics(meter metric.Meter) (*runMetrics, error) {
	m := &runMetrics{}
	m.lastWritten.Store(-1)

	var errs [8]error
	m.runs, errs[0] = meter.Int64Counter("collector.runs",
		metric.WithDescription("Collector runs, by result (success or failure)."),
		metric.WithUnit("{run}"))
	m.runDuration, errs[1] = meter.Float64Histogram("collector.run.duration",
		metric.WithDescription("Duration of a collector run, including storage and the MongoDB sync."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(runBuckets...))
	m.written, errs[2] = meter.Int64Counter("collector.articles.written",
		metric.WithDescription("New articles stored."),
		metric.WithUnit("{article}"))
	m.duplicates, errs[3] = meter.Int64Counter("collector.articles.duplicates",
		metric.WithDescription("Articles skipped as already stored, by match (url or title)."),
		metric.WithUnit("{article}"))
	m.fetched, errs[4] = meter.Int64Counter("collector.source.articles",
		metric.WithDescription("Articles fetched from a source, before deduplication."),
		metric.WithUnit("{article}"))
	m.failures, errs[5] = meter.Int64Counter("collector.source.failures",
		metric.WithDescription("Failed fetches from a source."),
		metric.WithUnit("{failure}"))

	_, errs[6] = meter.Int64ObservableGauge("collector.last_success.timestamp",
		metric.WithDescription("Unix time of the last successful run; absent until one succeeds."),
		metric.WithUnit("s"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			if ts := m.lastSuccess.Load(); ts > 0 {
				o.Observe(ts)
			}
			return nil
		}))
	_, errs[7] = meter.Int64ObservableGauge("collector.last_run.articles",
		metric.WithDescription("New articles stored by the last run; absent until a run stores."),
		metric.WithUnit("{article}"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			if n := m.lastWritten.Load(); n >= 0 {
				o.Observe(n)
			}
			return nil
		}))

	if err := errors.Join(errs[:]...); err != nil {
		return nil, err
	}

	// Start the run counters at 0. Prometheus's increase() needs a sample
	// before a counter's first increment to count it, and the startup run
	// often finishes before the first scrape.
	ctx := context.Background()
	for _, result := range []string{"success", "failure"} {
		m.runs.Add(ctx, 0, metric.WithAttributes(attribute.String("result", result)))
	}
	m.written.Add(ctx, 0)
	for _, match := range []string{"url", "title"} {
		m.duplicates.Add(ctx, 0, metric.WithAttributes(attribute.String("match", match)))
	}

	return m, nil
}

// recordSources counts each source's fetched articles and failures.
func (m *runMetrics) recordSources(ctx context.Context, sources []collector.SourceResult) {
	for _, sr := range sources {
		source := metric.WithAttributes(attribute.String("source", sr.Name))

		if sr.Error != "" {
			m.failures.Add(ctx, 1, source)
			// Register the series at 0 too, so rate() and increase() see
			// a source's first article after a failure.
			m.fetched.Add(ctx, 0, source)
			continue
		}

		m.failures.Add(ctx, 0, source)
		m.fetched.Add(ctx, int64(sr.Fetched), source)
	}
}

// recordStored counts what a run stored.
func (m *runMetrics) recordStored(ctx context.Context, manifest collector.Manifest) {
	m.written.Add(ctx, int64(manifest.ArticlesWritten))
	m.lastWritten.Store(int64(manifest.ArticlesWritten))
	m.duplicates.Add(ctx, int64(manifest.DuplicatesByURL), metric.WithAttributes(attribute.String("match", "url")))
	m.duplicates.Add(ctx, int64(manifest.DuplicatesByTitle), metric.WithAttributes(attribute.String("match", "title")))
}

// recordRun records a finished run that started at start; err is runOnce's
// result.
func (m *runMetrics) recordRun(ctx context.Context, start time.Time, err error) {
	end := time.Now()

	result := "success"
	if err != nil {
		result = "failure"
	} else {
		m.lastSuccess.Store(end.Unix())
	}

	m.runs.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
	m.runDuration.Record(ctx, end.Sub(start).Seconds(), metric.WithAttributes(attribute.String("result", result)))
}
