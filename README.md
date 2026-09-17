# Arium
Project to showcase my skills in frontend, backend, SRE and DevOps.

## Structure of the project

```text
backend/
├── cmd/
│   └── api/
│       └── main.go             # Application entry point
│
├── internal/
│   ├── config/
│   │   └── config.go           # Environment configuration
│   │
│   ├── database/
│   │   └── mongodb.go          # MongoDB connection
│   │
│   ├── user/
│   │   ├── model.go            # User and request/response types
│   │   ├── repository.go       # MongoDB operations
│   │   ├── service.go          # Business logic
│   │   ├── handler.go          # HTTP handlers
│   │   └── routes.go           # User routes
│   │
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── cors.go
│   │   └── logging.go
│   │
│   └── server/
│       └── server.go           # Router and HTTP server setup
│
├── pkg/
│   └── response/
│       └── response.go         # Shared JSON response helpers
│
├── migrations/                 # Optional data migration scripts
├── tests/
│   └── integration/
│
├── .env
├── .env.example
├── .gitignore
├── Dockerfile
├── docker-compose.yml
├── go.mod
└── go.sum
frontend/
devops/
├── docker/
├── jenkins/
└── kubernetes/
```
