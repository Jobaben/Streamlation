# Streamlation Implementation Plan

This plan synthesizes the architectural vision from `final-architectural-plan.md` and the phased execution guidance in `translation-streaming-plan.md`. It organizes the work into milestones, outlines key subsystems, and maps concrete tasks to responsible teams. The goal is to deliver a locally runnable, low-latency translation and casting experience while providing a clear path from MVP to production readiness.

---

## Retail Release Readiness Summary

The application is **not retail-ready**. Essential functionality—including the complete media translation pipeline, subtitle/audio output generation, robust persistence layers, authentication, and casting—remains incomplete. Quality gates such as integration testing, observability, compliance documentation, and installer packaging are still pending, so additional engineering phases must be completed before considering a retail launch.

### Implementation Closure Plan

| Workstream | Blockers | Target Deliverables | Implementation Location | Acceptance Evidence |
| --- | --- | --- | --- | --- |
| Media AI pipeline | Audio normalization, ASR/MT/TTS execution, and real output streaming are missing. Pipeline emits fake events via `SequentialStub`. | Production pipeline service with overlapping stages, FFmpeg normalization worker, Whisper ASR runner, MarianMT/Bergamot translator, Coqui/Bark TTS, subtitle assembler. | `packages/go/backend/media/normalize.go`, `packages/go/backend/asr/whisper.go`, `packages/go/backend/translation/marian.go`, `packages/go/backend/tts/coqui.go`, `packages/go/backend/output/subtitle.go`. Replace stub at `packages/go/backend/pipeline/pipeline.go:17-73` | End-to-end demo translating representative streams with latency telemetry; CI integration suite passing; pipeline runbooks published. |
| Platform resilience | Homegrown Postgres client at `packages/go/backend/postgres/client.go` concatenates SQL strings; Redis client at `packages/go/backend/redis/client.go` uses raw RESP without pooling. | `pgx` + migration tooling, resilient Redis client, connection pooling, structured retry/backoff policies. | Refactor `packages/go/backend/postgres/` to use `pgx`, add `packages/go/backend/postgres/migrations/`. Replace `packages/go/backend/redis/client.go` with `go-redis` wrapper. | Load test showing stable throughput, automated health checks in CI/CD, regression tests for failure cases. |
| UX & casting | Dashboard at `apps/web/app/page.tsx` lacks latency insights, casting flows, accessibility accommodations. | Next.js dashboard with translation controls, buffering indicators, Chromecast/AirPlay flows, accessibility audit fixes. | `apps/web/app/components/CastingControls.tsx`, `apps/web/app/components/LatencyIndicator.tsx`, `packages/go/backend/casting/chromecast.go`, `packages/go/backend/casting/airplay.go` | Playwright suite covering casting flows; WCAG-focused checklist complete. |
| Security & compliance | No enforced auth, auditing, or compliance documentation. | NextAuth + Go JWT integration, RBAC, audit logging schema, compliance guide. | `apps/api/cmd/server/auth.go`, `apps/api/cmd/server/middleware/auth.go`, `apps/web/app/api/auth/[...nextauth]/route.ts`, `docs/compliance.md` | Pen-test checklist signed off, retention purge APIs working, `docs/compliance.md` finalized. |
| Packaging & support | No installers, update channel, or support process. | Build scripts for macOS/Windows/Linux, model bundle distribution plan, support/rollback runbooks. | `build/installers/`, `build/models/`, `docs/deployment.md`, `docs/support-runbook.md` | Installer smoke tests recorded, support docs stored in repo, release checklist approved. |

---

## Current Repository Snapshot

### Implemented Components

| Component | Location | Status | Test Coverage |
| --- | --- | --- | --- |
| **Backend API** | `apps/api/cmd/server/` | Production-ready endpoints | Tested |
| - Health endpoint | `apps/api/cmd/server/main.go` | `GET /healthz` | Yes |
| - Session handlers | `apps/api/cmd/server/session.go` | CRUD operations | Yes |
| - WebSocket status | `apps/api/cmd/server/status.go` | Real-time streaming | Yes |
| **Worker** | `apps/worker/cmd/worker/main.go` | Goroutine pool with bounded concurrency | Tested |
| - Ingestion orchestrator | `apps/worker/cmd/ingestion/` | Stream warm-up validation | Tested |
| **Ingestion Adapters** | `packages/go/backend/ingestion/` | | |
| - HLS adapter | `packages/go/backend/ingestion/hls.go` | Playlist parsing, segment fetch | Tested |
| - RTMP adapter | `packages/go/backend/ingestion/rtmp.go` | RTMP protocol handling | Tested |
| - File adapter | `packages/go/backend/ingestion/file.go` | Local media replay | Tested |
| - Source interface | `packages/go/backend/ingestion/source.go` | Shared `StreamSource` contract | N/A |
| **Data Layer** | `packages/go/backend/` | | |
| - Session models | `packages/go/backend/session/session.go` | DTOs for translation sessions | N/A |
| - Postgres store | `packages/go/backend/postgres/store.go` | Session CRUD | Tested |
| - Postgres client | `packages/go/backend/postgres/client.go` | **Technical debt: manual SQL** | Tested |
| - Redis queue | `packages/go/backend/queue/queue.go` | LPUSH/BRPOP operations | Tested |
| - Redis status | `packages/go/backend/status/redis.go` | Pub/sub for events | Tested |
| - Redis client | `packages/go/backend/redis/client.go` | **Technical debt: raw RESP** | **Not tested** |
| **Pipeline** | `packages/go/backend/pipeline/pipeline.go` | `SequentialStub` emits fake events | Tested |
| **Schemas** | `packages/schemas/translation-session.schema.json` | JSON Schema validation | N/A |
| **Frontend** | `apps/web/app/page.tsx` | Dashboard with session form + status | **Not tested** |
| **Infrastructure** | | | |
| - Docker stack | `docker-compose.yml` | All services orchestrated | N/A |
| - CI pipeline | `.github/workflows/ci.yml` | Tests, lint, Docker builds | N/A |

### Unimplemented Components

| Component | Target Location | Blocker |
| --- | --- | --- |
| DASH adapter | `packages/go/backend/ingestion/dash.go` | Returns "not implemented" error in `apps/worker/cmd/ingestion/ingestor.go:97-98` |
| WebRTC adapter | `packages/go/backend/ingestion/webrtc.go` | Not started |
| Audio normalization | `packages/go/backend/media/normalize.go` | Not started |
| ASR service | `packages/go/backend/asr/whisper.go` | Stubbed in pipeline |
| Translation service | `packages/go/backend/translation/marian.go` | Stubbed in pipeline |
| TTS service | `packages/go/backend/tts/coqui.go` | Not started |
| Subtitle generator | `packages/go/backend/output/subtitle.go` | Not started |
| Authentication | `apps/api/cmd/server/auth.go` | Not started |
| Casting | `packages/go/backend/casting/` | Not started |
| Frontend tests | `apps/web/__tests__/` | Not started |

This snapshot informs the phase updates below by grounding planned work against what already exists. The analysis also surfaced near-term engineering leverage points captured in the gap analysis.

### Gap Analysis & Recommended Updates

| # | Gap | Current Location | Target Location | Priority |
|---|-----|-----------------|-----------------|----------|
| 1 | **Storage and Data Safety** – Custom Postgres executor concatenates SQL literals, risking injection and encoding bugs. | `packages/go/backend/postgres/client.go` | Replace with `jackc/pgx`, add migrations to `packages/go/backend/postgres/migrations/` | High |
| 2 | **Queue & Streaming Resilience** – Redis access via handcrafted RESP writers without reconnection, auth, or backpressure. | `packages/go/backend/redis/client.go`, `packages/go/backend/queue/queue.go`, `packages/go/backend/status/redis.go` | Replace with `redis/go-redis` in `packages/go/backend/redis/`, add circuit breakers | High |
| 3 | **Pipeline Orchestration** – Worker executes `SequentialStub` that serializes fake events instead of real pipeline stages. | `packages/go/backend/pipeline/pipeline.go:17-73` | Implement real stages in `packages/go/backend/asr/`, `packages/go/backend/translation/`, `packages/go/backend/output/` | Critical |
| 4 | **Async IO & Backpressure** – Each Redis operation opens new TCP connection. No pooling or pipelining. | `packages/go/backend/redis/client.go` | Implement connection pooling with `go-redis`, add backpressure metrics | High |
| 5 | **Observability & QA Depth** – Only structured logs via zap stub. No tracing, metrics, or integration tests. | `third_party/go.uber.org/zap/` | Add OpenTelemetry in `packages/go/backend/telemetry/`, Prometheus metrics, integration tests in `tests/integration/` | Medium |
| 6 | **Frontend Feedback Loop** – Dashboard doesn't surface retry/error states or websocket disconnection handling. | `apps/web/app/page.tsx` | Add error boundaries, retry UI in `apps/web/app/components/`, add tests in `apps/web/__tests__/` | Medium |
| 7 | **Media Coverage Gaps** – Missing DASH and WebRTC adapters. | `apps/worker/cmd/ingestion/ingestor.go:97-98` returns error | Implement `packages/go/backend/ingestion/dash.go`, `packages/go/backend/ingestion/webrtc.go` | Medium |

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
> - 🆕 Requires updates: Persistence hardening (`pgx` or equivalent), managed Redis client integration, pipeline parallelism/backpressure, OpenTelemetry traces/metrics, ingestion adapters for DASH/WebRTC, media normalization, AI runners, subtitle generation, and enhanced UI feedback states.
>
> **Next Focus:** Harden the persistence/queue layers while expanding ingestion coverage, introducing audio normalization, ASR/translation runners, and subtitle generation so pipeline events reflect actual media progress instead of stubbed stages. Prioritize asynchronous orchestration so multiple sessions can progress concurrently without blocking the ingestion loop.

**Objectives**

- 🆕 Deliver end-to-end ingestion → audio normalization → ASR → translation → subtitle output.
- ✅ Provide REST/WebSocket APIs for session control and live status updates.
- 🆕 Build a minimal UI for stream setup, translation language selection, and live subtitle display that surfaces retry/error telemetry.

**Detailed Task Plan**

| Epic | Task | Implementation Location | Owner | Dependencies | Definition of Done |
| --- | --- | --- | --- | --- | --- |
| Audio normalization | Implement FFmpeg-based normalization worker producing PCM chunks stored in Redis streams. | `packages/go/backend/media/normalize.go`, `packages/go/backend/media/ffmpeg.go` | Media team | Finalize chunk schema in `packages/schemas/audio-chunk.schema.json`. | Worker emits normalized chunk events; integration tests (`tests/pipeline/audio_normalization_test.go`) pass. |
| Audio normalization | Add health/metrics endpoints exposing lag and error counters. | `packages/go/backend/media/metrics.go`, `observability/rules.yaml` | Platform team | Normalization worker merged. | Prometheus metrics exported; alerts defined in `observability/rules.yaml`. |
| ASR | Build Whisper runner with GPU/CPU profiles including batching controls. | `packages/go/backend/asr/whisper.go`, `packages/go/backend/asr/runner.go` | Media team | Normalized audio stream available. | Runner translates sample corpora in CI; hardware tuning doc added to `docs/asr-selection.md`. |
| Translation | Implement MarianMT/Bergamot translator service with caching and batching. | `packages/go/backend/translation/marian.go`, `packages/go/backend/translation/cache.go` | Media team | Timestamped transcripts from ASR. | Translator returns aligned segments; load test demonstrates <3 s added latency. |
| Subtitle output | Create subtitle composer writing SRT/VTT plus WebSocket diff stream. | `packages/go/backend/output/subtitle.go`, `packages/go/backend/output/srt.go`, `packages/go/backend/output/vtt.go` | Media team | Translation segments ready. | Generated artifacts validated by Playwright visual snapshot; API returns downloadable files via `apps/api/cmd/server/subtitles.go`. |
| Persistence | Swap to `pgx`, add migrations via `golang-migrate`, and enforce DAO layer. | Refactor `packages/go/backend/postgres/client.go`, add `packages/go/backend/postgres/migrations/*.sql` | Platform team | None | CI migration job passes; CRUD regression tests cover new DAO. |
| Queue | Replace RESP helpers with `go-redis`, add backpressure + reconnection logic. | Refactor `packages/go/backend/redis/client.go`, `packages/go/backend/queue/queue.go` | Platform team | None | Chaos test dropping Redis demonstrates graceful retry; metrics for queue depth emitted. |
| Observability | Instrument OpenTelemetry tracing across API, worker, pipeline. | `packages/go/backend/telemetry/tracer.go`, `packages/go/backend/telemetry/metrics.go` | Platform team | Pipeline stages implemented. | Traces viewable in Jaeger; SLO dashboard screenshot stored in `docs/observability/README.md`. |
| Frontend | Surface pipeline stage status, error toasts, retry controls. | `apps/web/app/components/PipelineStatus.tsx`, `apps/web/app/components/ErrorToast.tsx` | Web team | API/worker telemetry endpoints ready. | Playwright tests (`tests/e2e/session-lifecycle.spec.ts`) green; accessibility audit passes. |

**Key Workstreams**

- **Stream Ingestion & Media Pipeline**
  - ✅ Implement adapters for HLS and RTMP under `packages/go/backend/ingestion/`, exposing a `StreamSource` interface with jitter buffers, reconnect policies, and per-source metrics.
  - ✅ Add ingestion adapter for static file uploads while unifying metrics into the pipeline ledger.
  - 🆕 Add ingestion adapters for DASH and WebRTC sources, expanding shared metrics coverage across transports.
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

**Key Workstreams & Tasks**

| Epic | Focus | Implementation Location | Tasks |
| --- | --- | --- | --- |
| Dubbing | High-fidelity translated audio | `packages/go/backend/tts/coqui.go`, `packages/go/backend/tts/bark.go`, `packages/go/backend/tts/alignment.go` | 1. Implement TTS microservice orchestrating Coqui/Bark voices. 2. Build audio alignment module leveraging subtitle timestamps. 3. Expose gRPC/REST endpoints for dubbed segments with caching + retry at `apps/api/cmd/server/dubbing.go`. |
| Casting | Device playback | `packages/go/backend/casting/chromecast.go`, `packages/go/backend/casting/airplay.go`, `apps/web/app/components/CastingManager.tsx` | 1. Deliver Chromecast CAF receiver app with OAuth-based device linking. 2. Ship AirPlay RAOP bridge with buffering controls. 3. Add frontend casting manager with device discovery and failure UX. |
| Security | Auth hardening | `apps/api/cmd/server/auth.go`, `apps/api/cmd/server/middleware/auth.go`, `apps/web/app/api/auth/[...nextauth]/route.ts` | 1. Wire NextAuth to Go JWT issuer with refresh/rotation. 2. Add role-based authorization checks on session APIs. 3. Instrument audit logging at `packages/go/backend/audit/logger.go` and admin review UI. |
| Latency mgmt | QoS | `packages/go/backend/qos/buffer.go`, `apps/web/app/components/LatencyControls.tsx` | 1. Implement adaptive buffering heuristics reading pipeline lag metrics. 2. Add operator controls for buffer targets per session. 3. Surface alerts when SLA breaches occur via `observability/alerts/latency.yaml`. |

**Exit Validation**

- Playwright scripted session covers dubbed playback + casting toggles.
- Load test with three concurrent streams demonstrates <5 s end-to-end latency delta.
- Security audit checklist closed with evidence captured in `docs/security-audit.md`.

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

| Theme | Deliverables | Implementation Location | Responsible Team |
| --- | --- | --- | --- |
| Model operations | GPU/CPU profile documentation, automated model download/verification scripts, staged language rollout playbooks. | `build/models/`, `scripts/download-models.sh`, `docs/hardware-profiles.md` | AI platform |
| Observability maturity | Production dashboards, alert runbooks, synthetic monitoring pipeline, failure scenario regression suite. | `observability/dashboards/`, `observability/alerts/`, `docs/runbooks/`, `tests/chaos/` | SRE |
| Privacy & compliance | Data retention tooling, compliance checklist completion, legal review sign-off, localized privacy policy. | `packages/go/backend/retention/purge.go`, `docs/compliance.md`, `docs/privacy-policy.md` | Security/Legal |
| Packaging & deployment | Cross-platform installers, offline bundle packaging, release/rollback automation, support knowledge base. | `build/installers/macos/`, `build/installers/windows/`, `build/installers/linux/`, `docs/deployment.md` | Release engineering |

**Exit Criteria**

- Monitoring dashboards and alerting in place with runbooks for major failure modes.
- Compliance documentation and privacy tooling reviewed with stakeholders.
- Installation/distribution artifacts validated across target deployment environments.

## Phase 5: Scale & Enterprise Readiness (Weeks 17-20)

**Objectives**

- Prepare Streamlation for multi-tenant enterprise deployments and regional compliance requirements.
- Optimize infrastructure costs with autoscaling policies and cold-start mitigation.
- Finalize rollout playbooks and SRE operational readiness.

**Key Workstreams**

| Focus Area | Planned Tasks | Implementation Location | Success Criteria |
| --- | --- | --- | --- |
| Multi-tenant architecture | 1. Partition data schemas by tenant with row-level security. 2. Build quota enforcement service. 3. Provide tenant administration UI. | `packages/go/backend/postgres/migrations/tenant_rls.sql`, `packages/go/backend/quota/`, `apps/web/app/admin/tenants/` | Pen-test verifying isolation, automated quota tests. |
| Scalability & resilience | 1. Define autoscaling policies for API/worker deployments. 2. Run chaos experiments covering Redis/Postgres outages. 3. Tune cold-start behavior for model loading. | `deploy/k8s/hpa.yaml`, `tests/chaos/`, `packages/go/backend/models/warmup.go` | Observed recovery within SLO; chaos reports archived. |
| Operations | 1. Publish on-call runbooks and incident response templates. 2. Conduct gamedays with recorded learnings. 3. Establish release/rollback checklist with sign-offs. | `docs/runbooks/`, `docs/incident-response.md`, `docs/release-checklist.md` | Ops leadership approval; post-gameday actions closed. |

**Exit Criteria**

- Verified tenant isolation with load and security testing artifacts.
- Autoscaling and chaos scenarios documented with remediation steps.
- Operations handbook approved by SRE leadership with rollout and rollback workflows.

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

| Risk | Impact | Current Location | Mitigation |
| --- | --- | --- | --- |
| GPU-dependent models exceed local hardware budgets | Pipeline stalls or quality degradation | `packages/go/backend/asr/`, `packages/go/backend/tts/` (to create) | Provide tiered model profiles (CPU-only, GPU-accelerated) and auto-detect capabilities via `packages/go/backend/models/detector.go`. |
| Stream format variability causes ingestion failures | Translation sessions fail to start | `packages/go/backend/ingestion/` | Maintain pluggable adapters with comprehensive integration tests and fallback buffering strategies. Add DASH/WebRTC adapters. |
| Latency creep across pipeline stages | User experience suffers | Pipeline stub at `packages/go/backend/pipeline/pipeline.go` | Instrument each stage with metrics via `packages/go/backend/telemetry/`, enable adaptive buffering, and iterate on model/queue tuning. |
| Licensing uncertainties for models/components | Deployment delays | N/A | Track licenses in `docs/licenses/` and obtain approvals early; prefer permissive models. |
| Casting interoperability issues | Inconsistent playback experience | `packages/go/backend/casting/` (to create) | Implement robust device discovery, provide manual fallback instructions, and gather telemetry per device type. |
| Homegrown Postgres/Redis protocol implementations | Connection edge cases or performance regressions | `packages/go/backend/postgres/client.go`, `packages/go/backend/redis/client.go` | Replace with `pgx` and `go-redis` before GA. Add integration tests in `tests/integration/`. |
| Manual SQL concatenation introduces injection risk | Security and data integrity issues | `packages/go/backend/postgres/client.go` | Adopt parameterized queries via `pgx` or `sqlc`, enforce input validation at API edges, and add static analysis (`gosec`) to CI. |
| Lack of per-stage backpressure controls | Queue storms starve ingestion or overload GPUs | `apps/worker/cmd/worker/main.go` | Implement bounded goroutine pools per stage in `packages/go/backend/pipeline/`, propagate context deadlines, expose queue depth metrics. |

---

## Phase Exit Deliverables Checklist

### Phase 1

- [x] Turborepo monorepo with backend/frontend packages and shared schemas.
- [x] Docker Compose stack with Postgres, Redis, API, worker, and Next.js services plus native start scripts.
- [x] CI pipelines executing linting and unit tests with artifacts published.

### Phase 2

| Deliverable | Status | Implementation Location |
| --- | --- | --- |
| Ingestion adapters (HLS, RTMP) | Done | `packages/go/backend/ingestion/hls.go`, `rtmp.go`, `file.go` |
| Audio normalization service | Not started | `packages/go/backend/media/normalize.go` |
| ASR + translation services | Not started | `packages/go/backend/asr/whisper.go`, `packages/go/backend/translation/marian.go` |
| Subtitle generator | Not started | `packages/go/backend/output/subtitle.go`, `srt.go`, `vtt.go` |
| Next.js UI for session setup | Done | `apps/web/app/page.tsx` |

### Phase 3

| Deliverable | Status | Implementation Location |
| --- | --- | --- |
| TTS dubbing pipeline | Not started | `packages/go/backend/tts/coqui.go`, `bark.go` |
| Casting frontend | Not started | `apps/web/app/components/CastingManager.tsx` |
| OAuth authentication | Not started | `apps/api/cmd/server/auth.go`, `apps/web/app/api/auth/` |

### Phase 4

| Deliverable | Status | Implementation Location |
| --- | --- | --- |
| Observability stack | Not started | `packages/go/backend/telemetry/`, `observability/` |
| Compliance documentation | Not started | `docs/compliance.md`, `docs/privacy-policy.md` |
| Distribution artifacts | Not started | `build/installers/`

---

## Scaffold Implementation for Testability

This section describes how to implement a fully testable scaffold that enables end-to-end testing without requiring real AI models, FFmpeg binaries, or external services. Each component should have both a production implementation and a testable stub.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           INTERFACE LAYER                                    │
│  Each component defines an interface that both real and stub implement       │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
          ┌─────────────────────────┼─────────────────────────┐
          ▼                         ▼                         ▼
   ┌─────────────┐          ┌─────────────┐          ┌─────────────┐
   │   REAL      │          │   STUB      │          │   MOCK      │
   │ Production  │          │ Deterministic│          │ Test-only   │
   │ FFmpeg/AI   │          │ Canned data │          │ Assertions  │
   └─────────────┘          └─────────────┘          └─────────────┘
```

### Component Scaffold Specifications

#### 1. Audio Normalizer

**Interface Location:** `packages/go/backend/media/normalizer.go`

```go
// Normalizer extracts and normalizes audio from media streams
type Normalizer interface {
    // Normalize processes a media source and emits PCM chunks
    Normalize(ctx context.Context, source io.Reader) (<-chan AudioChunk, error)
    // Health returns normalizer status
    Health() HealthStatus
}

type AudioChunk struct {
    Timestamp   time.Duration
    SampleRate  int
    Channels    int
    PCMData     []byte
    RMS         float64  // For testing assertions
}
```

**Production Implementation:** `packages/go/backend/media/ffmpeg_normalizer.go`
- Wraps FFmpeg binary via exec
- Streams PCM chunks to channel

**Stub Implementation:** `packages/go/backend/media/stub_normalizer.go`
- Reads from `testdata/audio/` sample files
- Emits deterministic chunks with predictable timestamps
- Configurable delay between chunks for latency testing

**Test Data:** `packages/go/backend/media/testdata/`
```
testdata/
├── audio/
│   ├── sample_en_10s.pcm      # 10 seconds English speech
│   ├── sample_es_10s.pcm      # 10 seconds Spanish speech
│   └── silence_5s.pcm         # 5 seconds silence
└── expected/
    ├── chunks_en_10s.json     # Expected chunk metadata
    └── timestamps.json        # Expected timing data
```

#### 2. ASR (Speech Recognition)

**Interface Location:** `packages/go/backend/asr/recognizer.go`

```go
// Recognizer transcribes audio chunks to text
type Recognizer interface {
    // Recognize processes audio and returns transcription
    Recognize(ctx context.Context, chunks <-chan AudioChunk) (<-chan Transcript, error)
    // LoadModel loads a specific model profile
    LoadModel(profile ModelProfile) error
    // Health returns recognizer status
    Health() HealthStatus
}

type Transcript struct {
    SessionID   string
    Text        string
    StartTime   time.Duration
    EndTime     time.Duration
    Confidence  float64
    Language    string
    Words       []Word  // Word-level timing for alignment
}

type Word struct {
    Text      string
    StartTime time.Duration
    EndTime   time.Duration
}
```

**Production Implementation:** `packages/go/backend/asr/whisper_recognizer.go`
- Wraps whisper.cpp or ONNX runtime
- GPU/CPU profile selection

**Stub Implementation:** `packages/go/backend/asr/stub_recognizer.go`
- Returns canned transcripts from `testdata/transcripts/`
- Maps input audio hashes to predetermined outputs
- Simulates processing delay (configurable)
- Can inject errors for failure testing

**Test Data:** `packages/go/backend/asr/testdata/`
```
testdata/
├── transcripts/
│   ├── en_hello_world.json    # "Hello world" transcript
│   ├── es_hola_mundo.json     # Spanish equivalent
│   └── multi_speaker.json     # Multiple speakers
└── mappings/
    └── audio_to_transcript.json  # Hash → transcript mapping
```

#### 3. Translation Service

**Interface Location:** `packages/go/backend/translation/translator.go`

```go
// Translator converts text between languages
type Translator interface {
    // Translate converts text to target language
    Translate(ctx context.Context, text string, sourceLang, targetLang string) (Translation, error)
    // TranslateStream processes streaming transcripts
    TranslateStream(ctx context.Context, transcripts <-chan Transcript, targetLang string) (<-chan Translation, error)
    // SupportedLanguages returns available language pairs
    SupportedLanguages() []LanguagePair
}

type Translation struct {
    SourceText    string
    TranslatedText string
    SourceLang    string
    TargetLang    string
    Confidence    float64
    Timestamp     time.Duration
}
```

**Production Implementation:** `packages/go/backend/translation/marian_translator.go`
- Wraps MarianMT or Bergamot
- Caching layer for repeated phrases

**Stub Implementation:** `packages/go/backend/translation/stub_translator.go`
- Dictionary-based translation from `testdata/dictionaries/`
- Deterministic outputs for testing
- Configurable latency simulation

**Test Data:** `packages/go/backend/translation/testdata/`
```
testdata/
├── dictionaries/
│   ├── en_es.json             # English → Spanish mappings
│   ├── en_fr.json             # English → French mappings
│   └── common_phrases.json    # Frequently used phrases
└── expected/
    └── translation_outputs.json
```

#### 4. Subtitle Generator

**Interface Location:** `packages/go/backend/output/generator.go`

```go
// SubtitleGenerator creates subtitle files from translations
type SubtitleGenerator interface {
    // GenerateSRT creates SRT format subtitles
    GenerateSRT(ctx context.Context, translations <-chan Translation) (io.Reader, error)
    // GenerateVTT creates WebVTT format subtitles
    GenerateVTT(ctx context.Context, translations <-chan Translation) (io.Reader, error)
    // StreamSubtitles provides real-time subtitle updates
    StreamSubtitles(ctx context.Context, translations <-chan Translation) (<-chan SubtitleEvent, error)
}

type SubtitleEvent struct {
    Type      string  // "add", "update", "remove"
    Index     int
    StartTime time.Duration
    EndTime   time.Duration
    Text      string
}
```

**Production Implementation:** `packages/go/backend/output/subtitle_generator.go`
**Stub Implementation:** `packages/go/backend/output/stub_generator.go`

#### 5. TTS (Text-to-Speech)

**Interface Location:** `packages/go/backend/tts/synthesizer.go`

```go
// Synthesizer converts text to speech audio
type Synthesizer interface {
    // Synthesize generates audio from text
    Synthesize(ctx context.Context, text string, voice VoiceProfile) (AudioSegment, error)
    // SynthesizeStream processes streaming translations
    SynthesizeStream(ctx context.Context, translations <-chan Translation, voice VoiceProfile) (<-chan AudioSegment, error)
    // AvailableVoices returns supported voice profiles
    AvailableVoices(lang string) []VoiceProfile
}

type AudioSegment struct {
    PCMData     []byte
    SampleRate  int
    Duration    time.Duration
    Timestamp   time.Duration  // Alignment with source
}
```

**Stub Implementation:** `packages/go/backend/tts/stub_synthesizer.go`
- Returns pre-recorded audio segments from `testdata/voices/`
- Maps text hashes to audio files

#### 6. Pipeline Orchestrator

**Interface Location:** `packages/go/backend/pipeline/runner.go`

```go
// Runner orchestrates the full translation pipeline
type Runner interface {
    // Run executes the pipeline for a session
    Run(ctx context.Context, session *Session, publisher StatusPublisher) error
    // Status returns current pipeline state
    Status(sessionID string) PipelineStatus
}

type PipelineStatus struct {
    Stage       string    // "ingestion", "normalization", "asr", "translation", "output"
    State       string    // "pending", "running", "completed", "failed"
    Progress    float64   // 0.0 - 1.0
    LastUpdate  time.Time
    Error       string
}
```

**Production Implementation:** `packages/go/backend/pipeline/production_runner.go`
- Wires real components together
- Manages concurrency and backpressure

**Stub Implementation (exists):** `packages/go/backend/pipeline/pipeline.go:SequentialStub`
- Currently emits fake events
- **Enhance to:** Wire stub components for realistic data flow

**Testable Implementation:** `packages/go/backend/pipeline/testable_runner.go`
- Wires all stub implementations together
- Produces real (deterministic) outputs
- Full end-to-end testability without external dependencies

### Dependency Injection Setup

**Location:** `packages/go/backend/di/container.go`

```go
// Container holds all service dependencies
type Container struct {
    Normalizer   media.Normalizer
    Recognizer   asr.Recognizer
    Translator   translation.Translator
    Synthesizer  tts.Synthesizer
    Generator    output.SubtitleGenerator
    Runner       pipeline.Runner
}

// NewProductionContainer creates container with real implementations
func NewProductionContainer(cfg Config) (*Container, error)

// NewTestContainer creates container with all stubs
func NewTestContainer() *Container

// NewMixedContainer allows mixing real and stub components
func NewMixedContainer(opts ...ContainerOption) *Container
```

### Test Harness

**Location:** `tests/harness/`

```
tests/
├── harness/
│   ├── harness.go             # Main test harness setup
│   ├── fixtures.go            # Test data loading
│   └── assertions.go          # Custom test assertions
├── integration/
│   ├── pipeline_test.go       # End-to-end pipeline tests
│   ├── api_test.go            # API integration tests
│   └── websocket_test.go      # WebSocket streaming tests
└── e2e/
    ├── session_lifecycle_test.go
    └── translation_flow_test.go
```

**Harness Usage:**

```go
func TestPipelineEndToEnd(t *testing.T) {
    // Create test container with stubs
    container := di.NewTestContainer()

    // Create test session
    session := &session.TranslationSession{
        ID:             "test-session-001",
        Source:         session.Source{Type: "file", URI: "testdata/sample.mp4"},
        TargetLanguage: "es",
    }

    // Run pipeline
    events := make(chan status.SessionStatusEvent, 100)
    publisher := status.NewChannelPublisher(events)

    err := container.Runner.Run(context.Background(), session, publisher)
    require.NoError(t, err)

    // Verify expected events
    harness.AssertEventsMatch(t, events, []string{
        "ingestion:running",
        "normalization:running",
        "asr:running",
        "translation:running",
        "output:running",
        "output:completed",
    })

    // Verify outputs
    subtitles := harness.LoadGeneratedSubtitles(t, session.ID)
    assert.Contains(t, subtitles, "Hola mundo")
}
```

### Environment Configuration

**Location:** `configs/`

```
configs/
├── production.yaml            # Real implementations
├── development.yaml           # Mix of real and stubs
├── testing.yaml               # All stubs
└── ci.yaml                    # CI-specific settings
```

**Example `testing.yaml`:**

```yaml
pipeline:
  normalizer: stub
  recognizer: stub
  translator: stub
  synthesizer: stub
  generator: stub

stubs:
  normalizer:
    delay_ms: 100
    chunk_size: 4096
  recognizer:
    delay_ms: 500
    error_rate: 0.0
  translator:
    delay_ms: 200
    dictionary: testdata/dictionaries/en_es.json
```

### Implementation Priority for Testability

| Priority | Component | Files to Create | Effort |
| --- | --- | --- | --- |
| 1 | DI Container | `packages/go/backend/di/container.go` | Low |
| 2 | Interface definitions | `packages/go/backend/*/interface.go` | Low |
| 3 | Stub implementations | `packages/go/backend/*/stub_*.go` | Medium |
| 4 | Test data | `packages/go/backend/*/testdata/` | Medium |
| 5 | Test harness | `tests/harness/` | Medium |
| 6 | Integration tests | `tests/integration/` | Medium |
| 7 | Testable runner | `packages/go/backend/pipeline/testable_runner.go` | Medium |

### Verification Checklist

After implementing the scaffold:

- [ ] `go test ./...` passes with all stubs
- [ ] `docker compose -f docker-compose.test.yml up` runs full stack with stubs
- [ ] Pipeline produces deterministic SRT/VTT output from test audio
- [ ] WebSocket streams deliver expected status events
- [ ] API endpoints return predictable responses
- [ ] CI can run full integration suite without external dependencies
- [ ] Test coverage reports show all paths exercised

---

## Plan Maintenance & Governance

- Review and update this plan at the conclusion of every task, noting whether affected phases are complete, partially complete, or require rework.
- Record adjustments to scope, timelines, or deliverables inside this document so downstream teams receive the latest guidance before starting new work.
- Escalate material risks or dependency changes during phase reviews and capture mitigations in the Risks & Mitigations section.

This phased implementation roadmap grounds the architectural blueprint in actionable milestones, ensuring cohesive progress toward a production-ready Streamlation experience.
