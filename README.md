# Streamlation

Watch and listen in your language.

A local-first streaming translation platform that ingests video/audio streams,
transcribes and translates spoken content using AI, and serves translated output
(subtitles and dubbed audio) in near real-time.

---

## Current Status

**Retail Readiness: NOT READY**

The orchestration foundation (Phase 1) is complete. The core AI translation
pipeline remains stubbed—no actual translation occurs yet.

| Phase | Description | Progress |
|-------|-------------|----------|
| Phase 1 | Foundation Infrastructure | 100% |
| Phase 2 | MVP Translation Pipeline | ~10% |
| Phase 3 | Enhanced Media Experience | 0% |
| Phase 4 | Production Hardening | 0% |
| Phase 5 | Enterprise Scale | 0% |

### What Works Today

- Session registration, persistence, and retrieval via REST API
- Real-time status streaming via WebSocket
- HLS, RTMP, and file-based stream ingestion with warm-up validation
- Worker job queue with bounded concurrency
- Next.js dashboard for session management and live monitoring
- Docker Compose stack for local development
- CI/CD pipeline with tests, linting, and container builds

### What's Missing (Critical Path)

| Component | Status | Implementation Location |
|-----------|--------|------------------------|
| Audio Normalization | Not implemented | `packages/go/backend/media/` (to create) |
| ASR Service (Whisper) | Stubbed | `packages/go/backend/asr/` (to create) |
| Translation Service | Stubbed | `packages/go/backend/translation/` (to create) |
| Subtitle Generation | Not implemented | `packages/go/backend/output/` (to create) |
| TTS Dubbing | Not implemented | `packages/go/backend/tts/` (to create) |
| DASH Adapter | Not implemented | `packages/go/backend/ingestion/dash.go` |
| WebRTC Adapter | Not implemented | `packages/go/backend/ingestion/webrtc.go` |
| Authentication | Not implemented | `apps/api/cmd/server/auth.go` (to create) |
| Production DB Client | Technical debt | Replace `packages/go/backend/postgres/` with `pgx` |
| Production Redis Client | Technical debt | Replace `packages/go/backend/redis/` with `go-redis` |

---

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

## Test Coverage

| Area | Status | Notes |
|------|--------|-------|
| Go Backend | 62% file coverage | All tests pass |
| Frontend | 0% | No tests implemented |
| Integration | Limited | Protocol-level mocks only |

**Tested Components:**
- `packages/go/backend/ingestion/` - HLS, RTMP, file adapters
- `packages/go/backend/pipeline/` - Pipeline execution
- `packages/go/backend/postgres/` - Database operations
- `packages/go/backend/queue/` - Redis queue operations
- `packages/go/backend/status/` - Status pub/sub
- `apps/api/cmd/server/` - API handlers
- `apps/worker/cmd/` - Worker and ingestion

**Untested Components:**
- `packages/go/backend/redis/client.go` - Redis client (186 lines)
- `apps/web/` - Entire frontend

## Documentation

- [Final Architectural Plan](docs/final-architectural-plan.md) - Architecture vision and design decisions
- [Baseline Plan](docs/translation-streaming-plan.md) - Task stubs and progress tracking
- [Implementation Plan](docs/implementation-plan.md) - Phased roadmap with detailed tasks

### Documentation Gaps

The following documentation is referenced but not yet created:
- `docs/pipeline-interfaces.md` - Stage interface contracts
- `docs/asr-selection.md` - ASR model evaluation
- `docs/compliance.md` - GDPR/DMCA compliance checklist
- `docs/deployment.md` - Production deployment guide
- `docs/API.md` - OpenAPI specification
