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
```

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

`cmd/api/main.go` wires everything by hand (no DI framework): loads config → connects Mongo → constructs a
`user.Repository` → `user.Service` → `user.Handler` → passes the handler into `server.New`. `server.go` builds
the chi router, mounts global middleware (logging, recoverer, timeout, CORS), and mounts feature routers under
`/api/v1/...`.

Each feature (currently just `user`) follows a fixed layering, all under `internal/<feature>/`:

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

### CI/CD

`.github/workflows/deploy-local.yaml` runs on push to `main` (self-hosted runner): `test` job spins up Mongo,
runs `gofmt`, `go vet`, `go build`, unit tests, and integration tests against it; `deploy` job (gated on `test`
passing) rebuilds and redeploys the Docker Compose stack in place. There is no separate frontend CI job yet.
