# Observability (`devops/observability/`)

Configuration for the metrics stack that Docker Compose runs next to the app (see [`devops/docker`](../docker)).
The Go API and the news collector expose OpenTelemetry metrics in the Prometheus format on port 9464. Prometheus
scrapes them, and Grafana shows them on dashboards that are provisioned from this directory.

```text
observability/
├── prometheus/prometheus.yml               # scrape jobs: api (backend:9464), collector (collector:9464), prometheus
└── grafana/
    ├── provisioning/
    │   ├── datasources/prometheus.yaml     # the Prometheus datasource, uid "prometheus"
    │   └── dashboards/arium.yaml           # loads ../dashboards into the "Arium" folder
    └── dashboards/
        ├── api.json                        # Arium API: RED metrics per route
        └── collector.json                  # Arium Collector: runs, articles, source failures
```

The metric names and labels are listed in [`backend/README.md`](../../backend/README.md#logs-and-metrics).

## Dashboards

**Arium API** (the Grafana home dashboard) has a `Route` filter and shows:

- overview stats: request rate, 5xx error ratio, p95 latency, requests in flight
- request rate by route and by status code
- 5xx and 4xx rates by route
- p50/p95/p99 latency, and p95 by route
- Go runtime: memory, goroutines, CPU

**Arium Collector** shows:

- time since the last successful run (orange after 8.5h, red after 10h, since runs are 8h apart by default), runs and failed runs
  in the range, and new articles in the last run and in the range
- runs by result, articles stored (new and duplicates) and run duration per hour
- failures and fetched articles per source, and a per-source table

Counter-based panels use `increase()`, which can't count a run that finishes before Prometheus first scrapes a
new container. That's usually the startup run after a deploy. The "since last success" and "new articles in last
run" panels read gauges, so they always reflect it.

## Editing

- **Dashboards:** Grafana reloads the JSON files every 30s, and UI edits can't be saved over provisioned
  dashboards. Build a change in the UI, export it with *Share → Export → Save to file*, and replace the file here.
  Keep the datasource `{"type": "prometheus", "uid": "prometheus"}`.
- **Prometheus:** check the file with `promtool check config prometheus/prometheus.yml` (the deploy job does too),
  then `docker compose kill -s SIGHUP prometheus` to reload it. The deploy job does this as well.

## Not here yet

Logs in Loki, traces in Tempo and SLO burn-rate alerts come next (see [`CLAUDE.md`](../../CLAUDE.md#next-steps)).
The collector's `collector_last_success_timestamp_seconds` is meant to back the staleness alert, and later to
replace the heartbeat-file healthcheck.
