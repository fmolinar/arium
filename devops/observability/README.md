# Observability (`devops/observability/`)

Configuration for the metrics, logs, traces and alerting stack that Docker Compose runs next to the app (see
[`devops/docker`](../docker)):

- **Metrics:** the Go API and the news collector expose OpenTelemetry metrics in the Prometheus format on port 9464.
  Prometheus scrapes them.
- **Logs:** Alloy ships every `arium-*` container's stdout to Loki.
- **Traces:** the API sends a trace of each request, including its MongoDB commands, to Tempo.
- **Alerts:** Prometheus evaluates SLO and availability rules and sends alerts to Alertmanager.
- **Grafana** reads all four, with logs and traces linked both ways.

```text
observability/
├── prometheus/
│   ├── prometheus.yml                      # scrape jobs, rule files, Alertmanager
│   └── rules/
│       ├── slo.rules.yml                   # SLO recording rules, burn-rate and availability alerts
│       └── tests/slo_test.yml              # promtool unit tests for those rules
├── alertmanager/alertmanager.yml           # routing; no notification integrations yet
├── loki/loki.yaml                          # single-process Loki, 14-day retention
├── tempo/tempo.yaml                        # single-process Tempo, OTLP/HTTP on :4318, 14-day retention
├── alloy/config.alloy                      # Docker logs → Loki, via docker-socket-proxy
└── grafana/
    ├── provisioning/
    │   ├── datasources/                    # prometheus, loki, tempo, alertmanager (uids match the names)
    │   └── dashboards/arium.yaml           # loads ../dashboards into the "Arium" folder
    └── dashboards/
        ├── api.json                        # Arium API: RED metrics per route
        ├── collector.json                  # Arium Collector: runs, articles, source failures
        └── slo.json                        # Arium SLOs: SLIs, error budgets, burn rates, API error logs
```

The metric names and labels, and what the API puts in its spans, are listed in
[`backend/README.md`](../../backend/README.md#logs-and-metrics).

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

**Arium SLOs** shows, for each SLO, the 30-day SLI, the error budget left, the burn rate over 1h/6h/1d/3d and
which routes are responsible. It also has API p99 and MongoDB p99 latency, and the backend's WARN and ERROR log
lines.

## Logs

Alloy discovers containers named `arium-*` through `docker-socket-proxy` and labels their lines with `container`
and `compose_service`. The API's and the collector's JSON lines also get a `level` label (`INFO`, `WARN`, ...) and
`trace_id` as structured metadata. Query them in Grafana's Explore with Loki, for example:

```logql
{compose_service="backend"} | json | msg="request" | status >= 500
{compose_service="collector", level="ERROR"}
{compose_service="backend"} | trace_id="<id>"
```

A line with a `trace_id` has a **View trace** link to Tempo. Container logs on disk are still rotated by Docker
(10 MB × 3); Loki keeps its copy for 14 days.

## Traces

In Explore with Tempo, search by service `arium-api`, or by span name such as `GET /api/v1/news`. A span's
**Logs for this span** link runs the Loki query for its trace ID. Tracing is on whenever the backend has
`OTEL_EXPORTER_OTLP_ENDPOINT` set, and every request is sampled, which is fine at this site's traffic. Set
`OTEL_TRACES_SAMPLER=parentbased_traceidratio` and `OTEL_TRACES_SAMPLER_ARG=0.1` to keep 10%.

## SLOs and alerts

Both SLOs cover all API requests over a 30-day window (which is why Prometheus keeps 30 days):

| SLO | Objective | Error budget |
|---|---|---|
| Availability | 99.5% of requests don't return a 5xx | 0.5%, about 3.6 hours of full outage a month |
| Latency | 99% of requests finish in 500 ms or less | 1% of requests |

They're measured by the API itself, so they can't see an outage that stops requests reaching it, such as the Mac
or the tunnel going down. `APIDown` catches a dead backend; a black-box probe of the public URL would catch the
rest.

Alerts use multiwindow, multi-burn-rate rules from the Google SRE Workbook. A burn rate of 1 spends the budget in
exactly 30 days.

| Severity | Burn rate | Long / short window | Budget spent | `for` |
|---|---|---|---|---|
| page | 14.4 | 1h / 5m | 2% in 1 hour | 2m |
| page | 6 | 6h / 30m | 5% in 6 hours | 15m |
| ticket | 3 | 1d / 2h | 10% in 1 day | 1h |
| ticket | 1 | 3d / 6h | 10% in 3 days | 3h |

Burn-rate alerts also need at least 20 requests in the long window, since with less traffic one error would look
like a burning budget. Alertmanager inhibits an SLO's tickets while it's paging.

| Alert | Fires when | Severity |
|---|---|---|
| `APIAvailabilityBudgetBurn` / `APILatencyBudgetBurn` | see above (labels `slo`, `window`) | page or ticket |
| `APIDown` | Prometheus can't scrape the API for 2 minutes | page |
| `CollectorDown` | Prometheus can't scrape the collector for 10 minutes | ticket |
| `CollectorStale` | no successful collection in over 10 hours | ticket |
| `CollectorNeverSucceeded` | no success in the 10 hours since the collector or Prometheus started | ticket |

Alerts show in Prometheus (`/alerts`), Alertmanager (http://localhost:9093) and Grafana's Alerting page. **Nothing
is sent anywhere yet:** Alertmanager's `default` receiver has no integrations. Add one (Discord, Slack, email) as
described in [`alertmanager.yml`](alertmanager/alertmanager.yml), keeping its secret in a file outside the repo.

### Responding

- **Availability burn:** on Arium SLOs, *5xx ratio by route* shows where the errors are. The logs panel below it,
  or `{compose_service="backend", level="ERROR"}`, shows what they are. Follow a line's trace link to see whether
  MongoDB was the slow or failing step. `/health` returning 503 means MongoDB is unreachable.
- **Latency burn:** *Slow ratio by route* and *MongoDB p99* tell an expensive route apart from a slow database. In
  Tempo, search for `{ duration > 500ms }` to find slow requests.
- **APIDown:** `docker compose ps backend` and `docker compose logs backend`.
- **Collector alerts:** the Arium Collector dashboard's per-source table, then `docker compose logs collector`.

## Editing

- **Dashboards:** Grafana reloads the JSON files every 30s, and UI edits can't be saved over provisioned
  dashboards. Build a change in the UI, export it with *Share → Export → Save to file*, and replace the file here.
  Keep the datasource uids (`prometheus`, `loki`).
- **Prometheus and rules:** from `devops/docker`, check and test them, then reload:

  ```sh
  docker compose run --rm --no-deps --entrypoint promtool prometheus check config /etc/prometheus/prometheus.yml
  docker compose run --rm --no-deps --entrypoint promtool prometheus test rules /etc/prometheus/rules/tests/slo_test.yml
  docker compose kill -s SIGHUP prometheus
  ```

  Changing a rule's labels or annotations means updating `slo_test.yml` too, since the tests match them exactly.
- **Alertmanager and Alloy** reload on `SIGHUP`. **Loki and Tempo** need `docker compose restart loki tempo`, and
  datasource changes need `docker compose restart grafana`.

The deploy job runs all of these checks before deploying, and reloads or restarts each service afterwards.

## Not here yet

- A notification integration for Alertmanager.
- Browser tracing: the frontend doesn't start traces, so a trace begins at the API.
- A black-box probe of the public URL, so the availability SLO sees outages in front of the API.
- Tempo's metrics generator (service graph, span metrics).
- Replacing the collector's heartbeat-file healthcheck with `collector_last_success_timestamp_seconds`.
