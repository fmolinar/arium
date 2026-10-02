package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/fmolinar/arium/backend/internal/collector"
)

func TestRunMetrics(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	m, err := newRunMetrics(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test"))
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	before := collect(t, reader)
	for _, name := range []string{"collector.last_success.timestamp", "collector.last_run.articles"} {
		if before[name] != nil {
			t.Errorf("%s reported before any run: %v", name, before[name])
		}
	}
	if n, ok := before["collector.runs"]["result=failure"]; !ok || n != 0 {
		t.Errorf("runs{result=failure} before any run = %d, %v; want 0, registered", n, ok)
	}

	m.recordSources(ctx, []collector.SourceResult{
		{Name: "feed-a", Fetched: 4},
		{Name: "feed-b", Error: "timeout"},
	})
	m.recordStored(ctx, collector.Manifest{ArticlesWritten: 3, DuplicatesByURL: 1, DuplicatesByTitle: 2})
	m.recordRun(ctx, time.Now(), nil)
	m.recordRun(ctx, time.Now(), errors.New("all sources failed"))

	got := collect(t, reader)

	want := map[string]map[string]int64{
		"collector.runs":                {"result=success": 1, "result=failure": 1},
		"collector.articles.written":    {"": 3},
		"collector.last_run.articles":   {"": 3},
		"collector.articles.duplicates": {"match=url": 1, "match=title": 2},
		"collector.source.articles":     {"source=feed-a": 4, "source=feed-b": 0},
		"collector.source.failures":     {"source=feed-a": 0, "source=feed-b": 1},
	}
	for name, series := range want {
		for attrs, value := range series {
			if got[name][attrs] != value {
				t.Errorf("%s{%s} = %d, want %d (all: %v)", name, attrs, got[name][attrs], value, got[name])
			}
		}
	}

	if ts := got["collector.last_success.timestamp"][""]; time.Since(time.Unix(ts, 0)) > time.Minute {
		t.Errorf("last success = %d, want about now", ts)
	}
	if n := got["collector.run.duration"]["result=success"] + got["collector.run.duration"]["result=failure"]; n != 2 {
		t.Errorf("run duration count = %d, want 2", n)
	}
}

// collect returns every int64 sum or gauge value, and every histogram's
// count, keyed by metric name and then by "key=value" attributes.
func collect(t *testing.T, reader sdkmetric.Reader) map[string]map[string]int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}

	out := map[string]map[string]int64{}
	add := func(name string, attrs attribute.Set, v int64) {
		if out[name] == nil {
			out[name] = map[string]int64{}
		}
		out[name][string(attrs.Encoded(attribute.DefaultEncoder()))] = v
	}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Sum[int64]:
				for _, dp := range data.DataPoints {
					add(m.Name, dp.Attributes, dp.Value)
				}
			case metricdata.Gauge[int64]:
				for _, dp := range data.DataPoints {
					add(m.Name, dp.Attributes, dp.Value)
				}
			case metricdata.Histogram[float64]:
				for _, dp := range data.DataPoints {
					add(m.Name, dp.Attributes, int64(dp.Count))
				}
			}
		}
	}

	return out
}
