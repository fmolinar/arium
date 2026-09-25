# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Arium is a portfolio project showcasing frontend, backend, SRE, and DevOps skills. It has two independently
versioned apps in one repo — a Go API (`backend/`) and a React app (`arium/`) — deployed together via Docker
Compose (`devops/docker/`).

## Commands

### Backend (`backend/`, Go 1.25)

```sh
cd backend
cp .env.example .env          # set JWT_SECRET at minimum
go run ./cmd/api               # run the API locally

go build ./...
go vet ./...
gofmt -l .                     # CI fails if this outputs anything

go test ./...                                          # unit tests (integration tests are skipped)
go test ./... -run TestName                             # single unit test
go test -tags=integration ./tests/integration/...        # integration tests, needs a reachable MongoDB

go run ./cmd/collector -dry-run                          # news collector: fetch live sources, print NDJSON only
go run ./cmd/collector                                   # run once, write to ./data (gitignored)
go run ./cmd/collector -schedule 00:00,08:00,16:00       # stay up, run daily at those times (-timezone, -once)
```

In Docker the `collector` service starts with the stack, runs once at startup and then on `COLLECTOR_SCHEDULE`
(default 3×/day UTC), writing to the `collector_data` volume. Its healthcheck (`collector -healthcheck`) reads a heartbeat file the
scheduler writes before each wait and fails once a run is more than 10 minutes overdue. One-off run:
`cd devops/docker && docker compose run --rm collector -once`.

Integration tests are gated behind the `integration` build tag so contributors without Docker aren't blocked
by `go test ./...`. To run them, start Mongo first:

```sh
docker run -d --rm -p 27017:27017 mongo:latest
```

### Frontend (`arium/`, React 19 + Vite)

```sh
cd arium
npm run dev        # dev server
npm run build       # production build
npm run lint         # eslint
npm run preview       # preview a production build
```

### Full stack (Docker Compose)

```sh
cd devops/docker
cp .env.example .env   # set JWT_SECRET
docker compose up -d --build
```

Runs `app` (React, host port 3000 → container 4173), `backend` (Go API, port 8080), and `mongo` (port 27017).
`JWT_SECRET` is required — compose refuses to start the backend without it.

## Architecture

### Backend request flow

`cmd/api/main.go` wires everything by hand (no DI framework): loads config → connects Mongo → for each feature
constructs `Repository` (and calls `EnsureIndexes`) → `Service` → `Handler` → passes the handlers into `server.New`. `server.go` builds
the chi router, mounts global middleware (logging, recoverer, timeout, CORS), and mounts feature routers under
`/api/v1/...`.

Each feature (`user`, `news`) follows a fixed layering, all under `internal/<feature>/`:

- `model.go` — domain type plus request/response DTOs
- `repository.go` — MongoDB access
- `service.go` — business logic (registration, login, profile updates)
- `handler.go` — HTTP handlers, translates between DTOs and service calls
- `routes.go` — mounts the feature's chi sub-router, applies `middleware.Auth` to protected routes only

New backend features should follow this same model/repository/service/handler/routes split and be mounted from
`server.go` the same way `user` is.

### Auth

JWT (HS256) issued by `middleware.NewToken` on login/register, carrying the user ID as `Subject` and a `Role`
custom claim. `middleware.Auth(cfg)` is applied per-route-group (see `user.Routes`), not globally — validates
the bearer token and injects the user ID/role into the request context, readable via `middleware.UserID(ctx)` /
`middleware.UserRole(ctx)`.

### Config

`internal/config.Load()` reads env vars once at startup (`ADDRESS`, `FRONTEND_URL`, `MONGO_URI`, `MONGO_DB`,
`JWT_SECRET`) and fails fast (`log.Fatal`) if `JWT_SECRET` is unset. `Config` is passed explicitly through the
call chain rather than accessed as a global.

### News collector

`cmd/collector` is a standalone CLI (separate from the API) that ingests DevOps/SRE/GitOps/DevSecOps news and
writes it to a directory, meant to be a Docker volume. With `-mongo-uri` (env `MONGO_URI`, set in Compose) each run
then syncs the stored articles into Mongo's `news` collection, which the API serves from `GET /api/v1/news`.

- Sources live in `internal/collector/<source>/`: `rss` (RSS/Atom feeds via gofeed) and `hackernews` (Algolia
  HN Search, title keyword queries, ≥20 points, only stories newer than `-since`). The default source list is
  `cmd/collector/sources.json`, embedded with `go:embed`; `-sources` points at a replacement. Each flag also has a
  `COLLECTOR_*` env var.
- `Collector.Collect` runs every `Source` concurrently, each with its own timeout and with panics recovered, then:
  `Normalize` (canonical URL, ID = first 16 hex chars of sha256(URL), plain-text title/summary, UTC) → drop
  articles older than `MaxAge` → deduplicate across sources → `Tagger.Tag` → `Persist`.
- `Persist` holds `Store.Lock` (a flock on `state/.lock`; `ErrLocked` if taken), writes raw payloads unless the
  sha256 matches the source's last stored one (`state/raw.json`), writes articles whose ID isn't in
  `state/seen.json` and whose `TitleKey` isn't in `state/titles.json`, runs `Store.Prune(retention)`, then writes
  the manifest.
- Retention (`-retention`, default 30 days) deletes day directories, manifests and state entries older than the
  cutoff. `-since` must be ≤ `-retention`, which the CLI enforces; otherwise pruned dedup state would let old
  articles back in. `TitleKey` returns "" for titles under 4 words, so generic titles are deduplicated by URL only.
- On-disk layout: `raw/<source>/<date>/<runID>.<ext>`, `articles/<date>/<runID>.ndjson`, `state/*.json`,
  `runs/<runID>.json` (the manifest, written last). Writes are atomic (temp file + rename), and source names and
  run IDs must pass `collector.ValidName`.
- Schedule mode (`-schedule`) is a loop around `Schedule.Next`, with no cron in the image. Missed runs aren't made
  up, and a failed run is logged without stopping the loop.
- A failed source is recorded in the manifest and the run continues. The CLI exits non-zero only when every
  source fails (`ErrAllSourcesFailed`), so a scheduler alerts on outages but not on one flaky feed.
- Delivery is at-least-once across crashes, so consumers should deduplicate by `id`.
- The Mongo sync (`cmd/collector/sync.go`) re-imports every article on the volume fetched within `-retention`,
  not just the run's new ones, via `news.Service.Import`: an upsert by `id` (the Mongo `_id`), then a delete of
  articles fetched before the cutoff. That makes it idempotent and lets a run that couldn't reach Mongo be caught up
  by the next. A sync failure fails the run. The `news` package owns the Mongo schema (snake_case bson, camelCase
  JSON); `cmd/collector` converts `collector.Article` to `news.Article`, so `internal/collector` never imports Mongo.
- `GET /api/v1/news?tag=&limit=&cursor=` is public, newest first, and pages with an opaque `nextCursor` over
  (`published_at`, `_id`) so ties on publish time don't skip or repeat articles.
- `collector.Article` uses the same JSON field names as the frontend news items, and tags are the slugs
  `devops|sre|gitops|devsecops` (`TOPICS` in `arium/src/api/news.js`).
- The frontend calls the API at relative `/api` URLs. `vite` and `vite preview` proxy them to `API_PROXY_TARGET`
  (default `http://localhost:8080`; `http://backend:8080` in Compose), so there's no API URL in the build.
  `NewsHub` pages through `useNews` (server-side tag filter, `nextCursor`), and `NewsTicker` fetches the 5 newest.
- In Compose the collector also runs once at startup (`COLLECTOR_RUN_ON_START=true`, flag `-run-on-start`), so a
  fresh stack gets news without waiting for the next slot. It writes a heartbeat due now before that run, so the
  healthcheck gives it the usual 10-minute grace.
- Hacker News is low-volume by design (expect 0–5 stories a week); the RSS feeds supply most articles.
- Collector tests never touch the network: they use `httptest.Server` with `testdata/` fixtures, and `t.TempDir()`
  for storage.

### CI/CD

`.github/workflows/deploy-local.yaml` runs on push to `main` (self-hosted runner): `test` job spins up Mongo,
runs `gofmt`, `go vet`, `go build`, unit tests, and integration tests against it; `deploy` job (gated on `test`
passing) rebuilds and redeploys the Docker Compose stack in place. There is no separate frontend CI job yet.

The workflow runs only on pushes to `main`, so PRs get no CI. `main` requires a code-owner review; the owner merges
with `gh pr merge --admin` (use `gh api -X PATCH repos/fmolinar/arium/pulls/<n> -f base=main` to retarget a PR,
since `gh pr edit` fails on a classic-Projects GraphQL error). Both self-hosted runners (`desktop-runner`,
`laptop-runner`) are often offline: runs then sit queued, and a newer push cancels the queued one.

## Next steps

Decisions already made (don't re-litigate): MongoDB stays the store for news (already deployed, document-shaped
data, a few thousand docs at most; no Postgres/Elasticsearch). Observability uses OpenTelemetry with the Grafana
stack, not ELK (Elasticsearch is too heavy for the Docker Desktop host, and ELK is log-centric).

1. **Structured logging.** Switch the API and collector to `log/slog` with JSON output, carrying the chi request
   ID and, once tracing lands, trace/span IDs. Replace the plain-text `middleware.Logging`.
2. **Metrics + Grafana.** Instrument with OpenTelemetry (Prometheus exporter or OTLP → Prometheus). API: RED
   metrics per route (rate, errors, duration). Collector: articles per run, per-source failures, run duration, and
   a last-successful-run timestamp (which should eventually replace the heartbeat-file healthcheck). Add Grafana with
   provisioned datasources and dashboards committed to the repo. `grafana/otel-lgtm` is fine to start with; split
   into separate Prometheus/Loki/Tempo/Grafana services later to show the production shape.
3. **Logs in Loki**, shipped from container stdout.
4. **Traces in Tempo**: OTel HTTP middleware on chi plus the MongoDB driver instrumentation, so a request is
   traceable browser → API → Mongo.
5. **SLOs and alerting**: availability/latency SLOs for the API, burn-rate alerts, and a collector staleness alert
   (no successful run in ~10h) via Grafana alerting or Alertmanager.

Also open: frontend lint/build in CI, a post-deploy `/health` check in the deploy job, and the planned
`devops/jenkins/` and `devops/kubernetes/` work (the collector's schedule maps onto a `CronJob`).
