# Streamlation

Watch and listen in your language.

Phase one establishes the foundational infrastructure for a local-first streaming
translation platform. The repository now contains a Turborepo workspace with Go
services, a Next.js frontend, shared JSON schemas, Docker orchestration, and CI
pipelines ready for subsequent feature phases.

## Repository Structure

```
apps/
  api/         # Go HTTP API with structured logging and health checks
  worker/      # Background job processor stub prepared for Redis queues
  web/         # Next.js frontend scaffolded with TypeScript
packages/
  schemas/     # JSON schemas shared between the API and frontend
third_party/
  go.uber.org/ # Local stub of zap logger to satisfy offline builds
```

## Prerequisites

- Go 1.22+
- Node.js 20+ (Corepack/PNPM recommended)
- Docker & Docker Compose (for the full local stack)

## Development

### Backend API

```bash
cd apps/api
go run ./cmd/server
```

Environment variables:

- `APP_SERVER_ADDR`: address for the HTTP server (default `:8080`)
- `APP_LOG_LEVEL`: `debug`, `info`, `warn`, or `error`

Endpoints:

- `GET /healthz`: health check used by local orchestration and CI.
- `POST /sessions`: validate and register a translation session using the shared schema defaults.
- `GET /sessions`: list recent sessions ordered by creation time.
- `GET /sessions/{id}`: retrieve a previously registered session definition.
- `GET /sessions/{id}/events` (WebSocket): stream real-time status updates for a session.

### Worker

```bash
cd apps/worker
go run ./cmd/worker
```

The worker consumes ingestion jobs from Redis, looks up session metadata, and
emits Redis-backed status events that the API streams to connected clients.

### Frontend

```bash
pnpm install
pnpm --filter @streamlation/web dev
```

The frontend renders a progress dashboard summarizing foundational milestones.

### Shared Schemas

JSON schemas live under `packages/schemas`. Both Go services and the frontend can
consume these definitions to validate session payloads.

## Docker Compose

Spin up the entire stack locally:

```bash
docker compose up --build
```

Services included:

- `postgres`: application database
- `redis`: queue and cache layer
- `api`: Go HTTP API listening on `localhost:8080`
- `worker`: background processor
- `web`: Next.js frontend served from `localhost:3000`

## Continuous Integration

The GitHub Actions workflow (`.github/workflows/ci.yml`) runs on pushes and pull
requests, executing Go tests, golangci-lint, PNPM lint/test tasks, and Docker image
builds for all services.

## Documentation

- [Final Architectural Plan](docs/final-architectural-plan.md)
- [Baseline Plan](docs/translation-streaming-plan.md)
- [Implementation Plan](docs/implementation-plan.md)
