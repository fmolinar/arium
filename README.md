# Arium
Project to showcase my skills in frontend, backend, SRE and DevOps.

## Components

Each part of the project has its own README describing its stack and how it works:

| Directory | What it is |
|---|---|
| [`arium/`](arium) | React 19 + Vite + Tailwind frontend |
| [`backend/`](backend) | Go API (users, JWT auth, news feed, MongoDB) and the news collector, with data-flow diagrams |
| [`devops/`](devops) | CI/CD pipeline (GitHub Actions, self-hosted runner) |
| [`devops/docker/`](devops/docker) | Dockerfiles and the Compose stack |

## Structure of the project

```text
backend/
├── cmd/
│   ├── api/
│   │   └── main.go             # Application entry point (config, Mongo, graceful shutdown)
│   └── collector/
│       ├── main.go             # News collector CLI (flags, runs sources, writes to a volume)
│       ├── sync.go             # Syncs stored articles into MongoDB after each run
│       ├── health.go           # Heartbeat file and -healthcheck
│       └── sources.json        # Default feed list and Hacker News queries (embedded)
│
├── internal/
│   ├── config/
│   │   └── config.go           # Env var loading (ADDRESS, FRONTEND_URL, MONGO_URI, MONGO_DB, JWT_SECRET)
│   │
│   ├── collector/              # News pipeline: normalize, tag, dedupe, atomic file store
│   │   ├── rss/                # RSS/Atom feed source
│   │   └── hackernews/         # Hacker News (Algolia search) source
│   │
│   ├── database/
│   │   └── database.go         # MongoDB connection
│   │
│   ├── user/
│   │   ├── model.go            # User model and request/response types
│   │   ├── repository.go       # MongoDB operations (unique email index)
│   │   ├── service.go          # Registration, login, profile logic
│   │   ├── handler.go          # HTTP handlers
│   │   └── routes.go           # User routes
│   │
│   ├── news/                   # News feed: same layering as user/, plus the Mongo import used by the collector
│   │
│   ├── middleware/
│   │   ├── auth.go             # JWT issuing/validation, route protection
│   │   ├── cors.go
│   │   └── logging.go
│   │
│   └── server/
│       └── server.go           # Router, HTTP server, health check
│
├── pkg/
│   └── response/
│       └── response.go         # Shared JSON response helpers
│
├── migrations/                 # Optional data migration scripts
├── tests/
│   └── integration/            # httptest-based API tests (needs a running Mongo, see below)
│
├── .env.example
├── go.mod
└── go.sum
frontend/
devops/
├── docker/
├── jenkins/
└── kubernetes/
```

### Backend

Go API (`backend/`) using [chi](https://github.com/go-chi/chi) and MongoDB.

**Implemented:**
- `GET /health` — reports API/Mongo status
- `POST /api/v1/users/register`, `POST /api/v1/users/login` — bcrypt-hashed passwords, issues an HS256 JWT
- `GET /api/v1/users/me`, `PATCH /api/v1/users/me` — JWT-protected profile read/update
- `GET /api/v1/news` — collected news, newest first; `?tag=`, `?limit=` (1–100) and cursor paging via `nextCursor`
- Graceful shutdown on SIGINT/SIGTERM
- Integration test suite in `tests/integration/` (see below)

CI (`.github/workflows/deploy-local.yaml`) runs gofmt, vet, build, unit and integration tests before every deploy.

**Running locally:**
```sh
cd backend
cp .env.example .env   # set JWT_SECRET at minimum
go run ./cmd/api
```

**Running the integration tests** (requires a reachable MongoDB):
```sh
docker run -d --rm -p 27017:27017 mongo:latest
cd backend
go test -tags=integration ./tests/integration/...
```
Plain `go test ./...` skips these (they're gated behind the `integration` build tag) so contributors without Docker aren't blocked.

### News collector

`backend/cmd/collector` is a CLI that pulls DevOps/SRE/GitOps/DevSecOps news from **RSS/Atom feeds** (Kubernetes,
CNCF, Docker, Argo, Flux, OpenTelemetry, Grafana, OpenSSF, Google Security) and **Hacker News**, normalizes and
tags it, writes it to a directory, and syncs it into MongoDB for the API:

```text
data/
├── raw/<source>/<date>/<runID>.xml|json   # upstream responses, verbatim; only stored when they changed
├── articles/<date>/<runID>.ndjson         # normalized articles first seen in this run
├── state/                                 # dedup state (seen URLs, titles, raw hashes) and run lock
└── runs/<runID>.json                      # per-run manifest: counts, duplicates, errors, pruning
```

**Run policy**
- **Schedule:** in Docker it runs **3 times a day, every day**, at 00:00, 08:00 and 16:00 UTC. Runs missed
  while the container is down are not made up.
- **Duplicates are not saved.** An article is skipped if its canonical URL was already stored, or if its title
  matches a stored one after lowercasing and removing punctuation and "Show HN:"-style prefixes. That catches
  the same story cross-posted under different URLs. Titles under 4 words ("v3.2.0", "Release notes") are only
  matched by URL. A raw payload identical to the last one stored for that source is skipped too.
- **Retention: 30 days.** After each run, raw and article files, manifests and dedup state older than 30 days
  are deleted. `-since` must be no longer than `-retention`, so a pruned article can't be picked up again.
- **MongoDB sync:** with `MONGO_URI` set (Compose sets it), each run upserts every article from the last 30 days
  into the `news` collection by `id` and deletes older ones. Re-importing everything is idempotent, and a run that
  couldn't reach MongoDB is caught up by the next one.
- A lock file stops a manual run and the scheduled one from writing at the same time; the second exits with an
  error.

```sh
cd backend
go run ./cmd/collector -dry-run   # fetch and print articles as NDJSON, write nothing
go run ./cmd/collector            # run once, write to ./data (gitignored)
go run ./cmd/collector -schedule 00:00,08:00,16:00   # stay up and run at these times daily
```

| Flag | Env var | Default | |
|---|---|---|---|
| `-out` | `COLLECTOR_OUT_DIR` | `./data` | output directory |
| `-sources` | `COLLECTOR_SOURCES` | built-in `sources.json` | custom feeds / HN queries |
| `-since` | `COLLECTOR_SINCE` | `168h` | skip older articles |
| `-timeout` | `COLLECTOR_SOURCE_TIMEOUT` | `15s` | per-source timeout |
| `-retention` | `COLLECTOR_RETENTION` | `720h` (30 days) | delete stored data older than this; `0` keeps everything |
| `-schedule` | `COLLECTOR_SCHEDULE` | empty (run once) | daily run times, `HH:MM,HH:MM,...` |
| `-timezone` | `COLLECTOR_TIMEZONE` | `UTC` | time zone for `-schedule` |
| `-mongo-uri` | `MONGO_URI` | empty (no sync) | sync stored articles into this MongoDB after each run |
| `-mongo-db` | `MONGO_DB` | `arium` | database for `-mongo-uri` |
| `-healthcheck` | | | exit non-zero if a scheduled run is over 10 minutes overdue |
| `-once` | | | run once even if a schedule is set |

A failing source is recorded in the run manifest without stopping the others. A single run exits non-zero only
when every source fails; in schedule mode, a failed run is logged and the next one still happens.

### Docker

`devops/docker/compose.yaml` runs the full stack: `app` (React, port 3000), `backend` (Go API, port 8080, built from `devops/docker/Dockerfile.backend`), and `mongo` (port 27017).

```sh
cd devops/docker
cp .env.example .env   # set JWT_SECRET
docker compose up -d --build
```
`JWT_SECRET` is required — compose refuses to start the backend without it rather than running with a broken/empty secret.

`up` also starts the news `collector`, which runs on the schedule above, writes to the `collector_data` named
volume and syncs articles into Mongo's `news` collection. Its healthcheck marks it unhealthy if a run is overdue. Change the run policy with the `COLLECTOR_*` variables in `.env` (see `.env.example`).
```sh
docker compose logs -f collector                                  # see runs and the next scheduled time
docker compose run --rm collector -once                           # collect once right now
docker run --rm -v docker_collector_data:/data alpine ls -R /data # inspect the volume
curl 'localhost:8080/api/v1/news?tag=sre&limit=5'                 # read the synced news through the API
```

## Roadmap

Next up: switching the frontend from mock data to `GET /api/v1/news`, then observability with OpenTelemetry and
the Grafana stack: structured logs, Prometheus metrics and dashboards, Loki, Tempo traces, and SLO-based alerting.
Details are in [`CLAUDE.md`](CLAUDE.md#next-steps).
