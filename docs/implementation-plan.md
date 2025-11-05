# Streamlation Implementation Plan

This plan synthesizes the architectural vision from `final-architectural-plan.md` and the phased execution guidance in `translation-streaming-plan.md`. It organizes the work into milestones, outlines key subsystems, and maps concrete tasks to responsible teams. The goal is to deliver a locally runnable, low-latency translation and casting experience while providing a clear path from MVP to production readiness.

---

## Current Repository Snapshot

- **Backend API (`apps/api`)** – Exposes health, session CRUD, and WebSocket streaming endpoints. Persists sessions through a homegrown Postgres client (`packages/go/backend/postgres`) that composes SQL strings manually via a thin executor abstraction, and publishes ingestion/status events over custom RESP helpers in `packages/go/backend/queue` and `packages/go/backend/status`.
- **Worker (`apps/worker`)** – Consumes Redis ingestion jobs, loads session metadata, and now distributes work across a configurable goroutine pool while still delegating to the sequential pipeline stub (`packages/go/backend/pipeline`). The worker emits richer lifecycle telemetry but continues to depend on handcrafted RESP helpers for Redis connectivity.
- **Media Ingestion (`packages/go/backend/ingestion`, `apps/worker/cmd/ingestion`)** – Provides production-ready HLS and RTMP adapters with jitter buffering, reconnection policies, and metrics counters plus an operator warm-up loop that validates stream availability before advancing the pipeline.
- **Shared Schemas & Packages** – JSON Schema definitions live in `packages/schemas`. Backend packages define shared session models, Redis pub/sub contracts, and a stub pipeline runner to keep API/worker integration tests hermetic.
- **Frontend (`apps/web`)** – Next.js dashboard for registering sessions, browsing persisted jobs, and monitoring live status streams. React Query caches session fetches, and server actions proxy API calls to validate backend wiring during development.
- **Tooling & Operations** – Docker Compose orchestrates Postgres, Redis, API, worker, and web services. GitHub Actions (`.github/workflows/ci.yml`) run Go unit tests, golangci-lint, PNPM lint/test, and container builds on every change. Makefile targets wrap `docker compose`, `pnpm`, and `go test` flows for contributors.

This snapshot informs the phase updates below by grounding planned work against what already exists. The analysis also surfaced near-term engineering leverage points captured in the gap analysis.

### Gap Analysis & Recommended Updates

1. **Storage and Data Safety** – The custom Postgres executor concatenates SQL literals and returns raw string slices. Introduce a battle-tested driver such as `jackc/pgx` (or at minimum parameterized statements) plus migration tooling to avoid injection risks, encoding bugs, and schema drift. Elevate row decoding to strongly typed structs and centralize connection pooling.
2. **Queue & Streaming Resilience** – Redis access is implemented via handcrafted RESP writers/readers without reconnection, authentication, or backpressure controls. Wrap these helpers behind an interface backed by a resilient client (e.g., `redis/go-redis`) and layer in circuit breakers, tracing, and health probes so ingestion pressure does not stall the worker.
3. **Pipeline Orchestration** – The worker now distributes ingestion jobs across a goroutine pool, but each job still executes the sequential stub that serializes ingestion, ASR, and translation. Promote a cancellable, event-sourced state machine so upcoming ASR/translation stages can overlap work, stream partial results, and emit granular progress once the media stack lands.
4. **Async IO & Backpressure** – Redis interactions hand-roll RESP commands and open a brand new TCP connection for every `LPUSH`, `BRPOP`, and `PUBLISH`, preventing connection reuse or pipelining. Replace these single-flight calls with pooled clients that can multiplex requests, stream pub/sub messages, and expose buffer pressure so the API can shed load instead of blocking request handlers.
5. **Observability & QA Depth** – Expand structured logging, metrics, and tracing across API and worker boundaries. Add golden integration tests that boot ephemeral Postgres/Redis containers, validate schema migrations, and assert end-to-end queue-to-websocket delivery to guard against protocol regressions.
6. **Frontend Feedback Loop** – The dashboard polls REST endpoints for history and listens to WebSockets for live status, but it does not surface retry/error states. Introduce Suspense-friendly data hooks, optimistic updates for session registration, and instrumentation for websocket disconnects to close the operator loop.
7. **Media Coverage Gaps** – The new ingestion adapters cover HLS and a simplified RTMP transport only. Expand coverage to DASH, progressive file sources, and WebRTC while layering in packet loss metrics and integration hooks for downstream normalization.

These updates are reflected in the phase adjustments below.

## Phase 1: Foundational Infrastructure (Weeks 1-3)

> **Status Update (current):** Phase 1 objectives are complete in the repository. The Turborepo workspace hosts Go services in `apps/api` and `apps/worker`, the Next.js frontend in `apps/web`, and shared packages in `packages/`. Docker Compose provisions Postgres, Redis, API, worker, and web services, and GitHub Actions exercises Go and frontend lint/test suites while building container images.
>
> **Delta vs. repo:** ✅ All Phase 1 objectives remain satisfied; no 🆕 requirements were introduced in this revision.

**Objectives**

- ✅ Establish the monorepo structure with Go backend and Next.js frontend workspaces.
- ✅ Set up local orchestration (Docker Compose) for Postgres, Redis, API, worker, and frontend.
- ✅ Implement continuous integration (linting, unit tests) and baseline observability (structured logging, health checks).

**Key Workstreams**

- **Platform & Tooling**
  - ✅ Initialize Turborepo with Go and Next.js packages; configure PNPM workspaces and Go modules.
  - ✅ Share schema definitions (JSON Schema or protobuf) between backend and frontend packages.
  - ✅ Author Dockerfiles for API, worker, frontend, Redis, and Postgres plus `docker-compose.yml` profiles for CPU-only and GPU-enabled environments.
  - ✅ Script native startup commands for users running outside containers.
- **Observability & Testing Foundations**
  - ✅ Configure GitHub Actions to run `golangci-lint`, Go unit tests, ESLint, Jest, and Docker image builds with published artifacts.
  - ✅ Adopt `uber-go/zap` for structured logging and expose baseline health checks.

**Exit Criteria**

- ✅ Monorepo scaffolding merged with automated lint/test pipelines.
- ✅ Local development stack (Docker Compose and native scripts) successfully spins up all core services.
- ✅ Basic observability (health checks, structured logs) operational in all services.

## Phase 2: MVP Translation Pipeline (Weeks 4-8)

> **Status Update (current):** Session lifecycle APIs, Redis-backed ingestion queueing, WebSocket status streaming, and the first media adapters are implemented. The Go API persists sessions to Postgres through manual SQL string construction, enqueues ingestion jobs, and proxies worker telemetry published through Redis. The worker consumes ingestion jobs via handcrafted RESP helpers, loads session metadata, fans work out across a goroutine pool, validates stream availability using the HLS/RTMP adapters, and then drives the sequential pipeline stub that emits stage events. The Next.js dashboard lets operators create sessions, inspect persisted metadata, and monitor live status streams.
>
> **Delta vs. repo:**
> - ✅ Covered today: Session CRUD, manual Postgres executor, handcrafted Redis queue/publish helpers, sequential pipeline stub, operator dashboard for creation + monitoring, plus production-ready HLS/RTMP ingestion adapters with worker warm-up handling.
> - 🆕 Requires updates: Persistence hardening (`pgx` or equivalent), managed Redis client integration, pipeline parallelism/backpressure, OpenTelemetry traces/metrics, ingestion adapters for DASH/file/WebRTC, media normalization, AI runners, subtitle generation, and enhanced UI feedback states.
>
> **Next Focus:** Harden the persistence/queue layers while expanding ingestion coverage, introducing audio normalization, ASR/translation runners, and subtitle generation so pipeline events reflect actual media progress instead of stubbed stages. Prioritize asynchronous orchestration so multiple sessions can progress concurrently without blocking the ingestion loop.

**Objectives**

- 🆕 Deliver end-to-end ingestion → audio normalization → ASR → translation → subtitle output.
- ✅ Provide REST/WebSocket APIs for session control and live status updates.
- 🆕 Build a minimal UI for stream setup, translation language selection, and live subtitle display that surfaces retry/error telemetry.

**Key Workstreams**

- **Stream Ingestion & Media Pipeline**
  - ✅ Implement adapters for HLS and RTMP under `packages/go/backend/ingestion/`, exposing a `StreamSource` interface with jitter buffers, reconnect policies, and per-source metrics.
  - 🆕 Add ingestion adapters for DASH, WebRTC, and static file uploads while unifying metrics into the pipeline ledger.
  - 🆕 Wrap FFmpeg/libav in `services/media/` to normalize audio to 16 kHz mono PCM chunks, tagging each frame with presentation timestamps and waveform statistics for downstream VAD.
  - 🆕 Introduce a `ChunkLedger` abstraction that records enqueue/dequeue offsets in Postgres so retries resume idempotently after worker restarts.
  - 🆕 Use `hibiken/asynq` (or Temporal/Argo Workflows if GPU orchestration demands) to parallelize ASR → translation tasks, persisting deterministic stage transitions in Postgres and caching transient state in Redis.
- **Pipeline Orchestration & Async Coordination**
  - ✅ Spin up a worker pool that pulls jobs concurrently, launching a goroutine per session with bounded concurrency and context cancellation so long-running ASR does not starve new ingests (landed in `apps/worker/cmd/worker`).
  - 🆕 Decouple status publication from hot paths by buffering events onto an internal channel serviced by dedicated publishers that can batch or drop on overload.
  - 🆕 Stream Redis pub/sub events via long-lived connections managed by health-checked pools, exposing an async iterator interface for WebSocket handlers and frontend consumers.
- **AI Services**
  - 🆕 Evaluate Whisper variants vs. alternatives; document selection in `docs/asr-selection.md` and implement GPU-aware loading with CPU fallbacks.
  - 🆕 Package MarianMT/Bergamot models per language pair with configurable latency vs. accuracy presets, and introduce a pluggable translation interface that supports batching for throughput gains.
- **Persistence & Platform Hardening**
  - 🆕 Replace custom SQL string concatenation with parameterized queries using `pgx` (or migrate to `gorm`/`sqlc`) and seed `golang-migrate` migrations for reproducible schemas.
  - 🆕 Swap handcrafted Redis RESP usage with a managed client that provides pooling, TLS/auth, and observable retries; expose health endpoints that validate queue depth and pub/sub connectivity.
  - 🆕 Capture OpenTelemetry traces for API handlers, queue events, and worker stages; emit cardinality-bounded metrics for session throughput and stage latency.
- **Output Generation & Delivery**
  - 🆕 Generate SRT/VTT artifacts in `services/output/`, annotate them with segment confidence scores, and expose subtitle buffers via WebSocket APIs plus HTTP range fetch for archival playback.
- **Frontend Integration**
  - ✅ Create Next.js pages/components for stream configuration, language selection, and live subtitle dashboards.
  - 🆕 Surface websocket retry state, ingestion lag metrics, and model selection guidance in the operator console using server actions and background revalidation.
- **Session Control APIs**
  - ✅ `POST /sessions` validates payloads against shared schema expectations and registers sessions in memory for MVP coordination.
  - ✅ `GET /sessions/{id}` retrieves stored session configurations for downstream pipeline stages.
  - ✅ Persist sessions to Postgres and emit ingestion jobs so the worker can start pulling media for translation.
  - ✅ Add WebSocket session status updates surfaced from Redis-backed worker progress events.
  - ✅ Implement media ingestion adapters that warm up sessions before pipeline execution (available under `apps/worker/cmd/ingestion`).
  - 🆕 Add audio normalization stubs that hand off work to the ASR stage.
  - ✅ Provide a worker-run pipeline stub that replays canonical stage events so the frontend can exercise status streaming end to end.

**Exit Criteria**

- 🆕 Demonstrable live session translating audio to subtitles with acceptable latency targets.
- 🆕 Multiple concurrent sessions advance through ingestion, ASR, and translation without head-of-line blocking or dropped status updates under nominal load.
- 🆕 Operator UI controlling session lifecycle and language selection backed by authenticated REST/WebSocket APIs.
- 🆕 Integration tests covering ingestion through subtitle delivery pass reliably in CI.

## Phase 3: Enhanced Media Experience (Weeks 9-12)

> **Adjustment:** The dubbing, casting, and authentication workstreams now depend on durable session storage arriving from Phase 2.
> Ensure the Phase 2 pick-up items above are complete before scheduling Phase 3 execution.

**Objectives**

- Add TTS-based dubbed audio output with synchronization controls.
- Integrate casting (Chromecast/AirPlay) and latency buffering controls on the frontend.
- Harden authentication with OAuth providers and JWT lifecycle management.

**Key Workstreams**

- **AI Services Enhancements**
  - Integrate Coqui TTS/Bark for adaptive voice cloning with fallback multi-speaker models and optional voice enrollment.
- **Output Generation & Delivery**
  - Produce translated audio segments accessible via REST or WebSocket push, ensuring alignment with subtitles.
- **Frontend Experience**
  - Overlay dubbed audio controls, integrate Video.js with HLS.js, and add casting controls leveraging Chromecast CAF receiver and AirPlay libraries.
- **Security, Privacy & Compliance**
  - Implement email/password and OAuth (Google, Apple, GitHub) flows via NextAuth.js backed by Go JWT APIs.
  - Secure device tokens for casting hardware and enforce rate limiting across session APIs.

**Exit Criteria**

- Users can opt into dubbed audio with synchronization controls and manage casting sessions from the UI.
- Authenticated experiences with OAuth/JWT lifecycles verified through integration tests.
- Latency buffering and device telemetry captured for casting scenarios.

## Phase 4: Production Hardening (Weeks 13-16)

**Objectives**

- Expand language packs, add GPU-aware model management, and document hardware presets.
- Implement comprehensive monitoring dashboards, alerting, and compliance tooling.
- Package installers and publish deployment guides for offline-first distribution.

**Key Workstreams**

- **AI Services Operations**
  - Manage model lifecycle with GPU-aware scheduling, caching, and documented hardware requirements.
  - Expand language availability with staged rollouts and quality benchmarks.
- **Observability & Testing Maturity**
  - Expose advanced metrics and integrate Prometheus/Grafana dashboards; define alert thresholds for stream drops, queue backlogs, and translation latency breaches.
  - Extend test suites with scenario tests mocking AI stages and Playwright end-to-end coverage.
- **Security, Privacy & Compliance**
  - Provide APIs/CLI commands to purge transcripts, audio buffers, and user sessions; document privacy controls and retention policies.
  - Track third-party model licenses, capture acknowledgements, and complete `docs/compliance.md`.
- **Distribution & Deployment**
  - Package installers, publish deployment guides for offline-first distribution, and document hardware presets.

## Phase 5: Scale & Enterprise Readiness (Weeks 17-20)

**Objectives**

- Prepare Streamlation for multi-tenant enterprise deployments and regional compliance requirements.
- Optimize infrastructure costs with autoscaling policies and cold-start mitigation.
- Finalize rollout playbooks and SRE operational readiness.

**Key Workstreams**

- **Multi-Tenant Architecture**
  - Introduce organization-level RBAC, workspace isolation, and configurable quota management.
- **Scalability & Resilience**
  - Define autoscaling rules for ingestion workers, media processors, and AI services; exercise chaos testing for fault tolerance.
- **Operational Runbooks**
  - Produce on-call guides, incident response templates, and upgrade/rollback procedures validated through gamedays.

**Exit Criteria**

- Verified tenant isolation with load and security testing artifacts.
- Autoscaling and chaos scenarios documented with remediation steps.
- Operations handbook approved by SRE leadership with rollout and rollback workflows.

**Exit Criteria**

- Monitoring dashboards and alerting in place with runbooks for major failure modes.
- Compliance documentation and privacy tooling reviewed with stakeholders.
- Installation/distribution artifacts validated across target deployment environments.

## Cross-Phase Dependencies & Sequencing Considerations

- Complete platform tooling before onboarding pipeline work to ensure shared schemas and CI guardrails.
- Deliver the ingestion and audio normalization layers prior to ASR/translation integration to supply consistent inputs.
- Land MVP UI concurrently with API development to validate realtime interactions early.
- Introduce voice cloning, casting, and compliance features only after the core translation loop is stable and monitored.
- Schedule hardware-specific optimizations (GPU presets, installers) after functional milestones to avoid premature optimization.
- Align persistence hardening (pgx migration, migrations, Redis client swap) before introducing stateful AI workloads so pipeline reliability improvements compound rather than overlap.
- Gate rollout of GPU-heavy services behind queue saturation SLAs and stage latency dashboards to prevent cascading failures.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| GPU-dependent models exceed local hardware budgets | Pipeline stalls or quality degradation | Provide tiered model profiles (CPU-only, GPU-accelerated) and auto-detect capabilities. |
| Stream format variability causes ingestion failures | Translation sessions fail to start | Maintain pluggable adapters with comprehensive integration tests and fallback buffering strategies. |
| Latency creep across pipeline stages | User experience suffers | Instrument each stage with metrics, enable adaptive buffering, and iterate on model/queue tuning. |
| Licensing uncertainties for models/components | Deployment delays | Track licenses in `docs/licenses/` and obtain approvals early; prefer permissive models. |
| Casting interoperability issues | Inconsistent playback experience | Implement robust device discovery, provide manual fallback instructions, and gather telemetry per device type. |
| Homegrown Postgres/Redis protocol implementations diverge from upstream behavior | Connection edge cases or performance regressions in production | Expand integration coverage against real services, add fuzz/integration tests for protocol handlers, and evaluate migration to maintained clients before GA. |
| Manual SQL concatenation introduces injection/encoding risk | Security and data integrity issues | Adopt parameterized queries via `pgx` or `sqlc`, enforce input validation at API edges, and add static analysis (`gosec`) to CI. |
| Lack of per-stage backpressure controls in workers | Queue storms starve ingestion or overload ASR/translation GPUs | Implement bounded goroutine pools per stage, propagate context deadlines, and expose queue depth metrics with autoscaling hooks. |

---

## Phase Exit Deliverables Checklist

### Phase 1

- [x] Turborepo monorepo with backend/frontend packages and shared schemas.
- [x] Docker Compose stack with Postgres, Redis, API, worker, and Next.js services plus native start scripts.
- [x] CI pipelines executing linting and unit tests with artifacts published.

### Phase 2

- [x] Ingestion adapters with automated tests for HLS and RTMP sample streams.
- [ ] Audio normalization service producing timestamped PCM chunks via Redis queues.
- [ ] ASR + translation services with documented model selection and cached translations.
- [ ] Subtitle generator delivering synchronized SRT/VTT outputs via WebSocket APIs.
- [x] Minimal Next.js UI for session setup, language controls, and live subtitles.

### Phase 3

- [ ] TTS dubbing pipeline providing synchronized audio output options.
- [ ] Casting-ready frontend with latency buffering controls and device telemetry.
- [ ] OAuth-backed authentication and session rate limiting validated via tests.

### Phase 4

- [ ] Observability stack (dashboards, alerts, runbooks) covering pipeline stages.
- [ ] Compliance documentation, privacy controls, and data lifecycle tooling.
- [ ] Distribution artifacts and deployment guides for offline-first installations.

## Plan Maintenance & Governance

- Review and update this plan at the conclusion of every task, noting whether affected phases are complete, partially complete, or require rework.
- Record adjustments to scope, timelines, or deliverables inside this document so downstream teams receive the latest guidance before starting new work.
- Escalate material risks or dependency changes during phase reviews and capture mitigations in the Risks & Mitigations section.

This phased implementation roadmap grounds the architectural blueprint in actionable milestones, ensuring cohesive progress toward a production-ready Streamlation experience.
