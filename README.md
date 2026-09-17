# Arium
Project to showcase my skills in frontend, backend, SRE and DevOps.

## Structure of the project

```text
backend/
├── cmd/
│   └── api/
│       └── main.go             # Application entry point (config, Mongo, graceful shutdown)
│
├── internal/
│   ├── config/
│   │   └── config.go           # Env var loading (ADDRESS, FRONTEND_URL, MONGO_URI, MONGO_DB, JWT_SECRET)
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

**Not yet implemented:** a backend Dockerfile/docker-compose service (only the frontend is containerized so far, under `devops/docker/`), and CI wiring for backend builds/tests.

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
