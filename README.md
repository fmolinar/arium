# Arium
Project to showcase my skills in frontend, backend, SRE and DevOps.

## Structure of the project

```text
backend/
├── cmd/
│   ├── api/
│   │   └── main.go             # Application entry point (config, Mongo, graceful shutdown)
│   └── collector/
│       ├── main.go             # News collector CLI (flags, runs sources, writes to a volume)
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
tags it, and writes it to a directory:

```text
data/
├── raw/<source>/<date>/<runID>.xml|json   # upstream responses, verbatim
├── articles/<date>/<runID>.ndjson         # normalized articles first seen in this run
├── state/seen.json                        # article IDs already written (dedup across runs)
└── runs/<runID>.json                      # per-run manifest: counts and errors per source
```

```sh
cd backend
go run ./cmd/collector -dry-run   # fetch and print articles as NDJSON, write nothing
go run ./cmd/collector            # write to ./data (gitignored)
```

| Flag | Env var | Default | |
|---|---|---|---|
| `-out` | `COLLECTOR_OUT_DIR` | `./data` | output directory |
| `-sources` | `COLLECTOR_SOURCES` | built-in `sources.json` | custom feeds / HN queries |
| `-since` | `COLLECTOR_SINCE` | `168h` | skip older articles |
| `-timeout` | `COLLECTOR_SOURCE_TIMEOUT` | `15s` | per-source timeout |

A failing source is recorded in the run manifest without stopping the others; the process exits non-zero only
when every source fails.

### Docker

`devops/docker/compose.yaml` runs the full stack: `app` (React, port 3000), `backend` (Go API, port 8080, built from `devops/docker/Dockerfile.backend`), and `mongo` (port 27017).

```sh
cd devops/docker
cp .env.example .env   # set JWT_SECRET
docker compose up -d --build
```
`JWT_SECRET` is required — compose refuses to start the backend without it rather than running with a broken/empty secret.

The news collector is a one-shot job behind the `collector` profile, so `up` doesn't start it. It writes to the
`collector_data` named volume:
```sh
docker compose --profile collector run --rm collector            # collect once
docker compose --profile collector build collector               # rebuild after code changes
docker run --rm -v docker_collector_data:/data alpine ls -R /data # inspect the volume
```
