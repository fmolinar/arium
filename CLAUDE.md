# CLAUDE.md

Arium is a portfolio project showing frontend, backend, SRE and DevOps work: a DevOps/SRE/GitOps/DevSecOps news hub with user accounts and (planned) a forum.

## Layout

- `arium/` — React 19 + Vite + Tailwind v4 frontend. (The README tree says `frontend/`; the real directory is `arium/`.)
- `backend/` — Go module `github.com/fmolinar/arium/backend` (chi, mongo-driver v2, golang-jwt v5).
  - `cmd/api` — HTTP API entry point.
  - `internal/<domain>/` — one package per domain, split into `model.go`, `repository.go` (Mongo), `service.go` (logic), `handler.go` (HTTP), `routes.go`. Follow this split for new domains.
  - `internal/collector/` — news collection pipeline (normalize, tag, store to disk). The `cmd/collector` binary and its sources are in progress.
  - `pkg/response` — use `response.JSON` / `response.Error` for all HTTP responses.
  - `tests/integration/` — httptest suite against a real Mongo, behind the `integration` build tag.
- `devops/docker/` — `compose.yaml` (app :3000, backend :8080, mongo :27017) and Dockerfiles. `devops/jenkins` and `devops/kubernetes` are empty placeholders.
- `.github/workflows/deploy-local.yaml` — on push to `main`, runs on a **self-hosted** runner: gofmt, vet, build, unit and integration tests, then `docker compose up` on the same machine.

## Commands

Backend (run from `backend/`):
```sh
gofmt -l .                     # CI fails if this prints anything
go vet ./...
go test ./...                  # unit tests only; no Docker needed
docker run -d --rm -p 27017:27017 mongo:latest
go test -tags=integration ./tests/integration/...   # TEST_MONGO_URI overrides the default mongodb://localhost:27017
JWT_SECRET=dev go run ./cmd/api
```

Frontend (run from `arium/`): `npm run dev`, `npm run lint`, `npm run build`.

Full stack: `cd devops/docker && cp .env.example .env` (set `JWT_SECRET`), then `docker compose up -d --build`.

## Conventions and decisions

- **Auth is JWT (HS256, 24h), not sessions.** This is deliberate; don't suggest switching. The token carries the user ID as `sub` plus a `role` claim. `middleware.Auth` puts both in the request context (`middleware.UserID`, `middleware.UserRole`).
- Config comes only from env vars (`internal/config`). `JWT_SECRET` is required, and both the API and compose refuse to start without it.
- Error strings are lowercase and wrapped with `%w`. Repositories expose sentinel errors (`ErrNotFound`, …) that handlers map to status codes with `errors.Is`.
- CI's test Mongo uses host port **27018** so it doesn't collide with the deployed stack's 27017 on the same runner. Keep it that way.
- Frontend news items have the shape `{id, title, summary, source, url, tags, publishedAt}`, and tags are the slugs `devops|sre|gitops|devsecops` (see `arium/src/data/mockNews.js`). `collector.Article` uses the same JSON field names, so the frontend can consume its output directly.

## Collector (`internal/collector`)

- Pipeline: a `Source` returns the raw payload and its articles, then `Normalize` (canonical URL, ID = first 16 hex chars of sha256(URL), plain-text title/summary, UTC), then `Tagger.Tag`, then `Store`.
- On-disk layout under the output root: `raw/<source>/<date>/<runID>.json`, `articles/<date>/<runID>.ndjson`, `state/seen.json`, `runs/<runID>.json`. Writes are atomic (temp file + rename). Source names and run IDs must pass `ValidName`.
- Delivery is at-least-once across crashes, so consumers should deduplicate by `id`. Only one run may use a given output root at a time.
- Tests must not touch the network: use `httptest.Server` with files in `testdata/`, and `t.TempDir()` for storage.

## Git

- Branch off `main` (`feature/...`, `fix/...`) and open a PR. Pushing to `main` deploys.
