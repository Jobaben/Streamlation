# Streamlation Implementation Plan

This plan synthesizes the architectural vision from `final-architectural-plan.md` and the phased execution guidance in `translation-streaming-plan.md`. It organizes the work into milestones, outlines key subsystems, and maps concrete tasks to responsible teams. The goal is to deliver a locally runnable, low-latency translation and casting experience while providing a clear path from MVP to production readiness.

---

## Phase 1: Foundational Infrastructure (Weeks 1-3)

**Objectives**

- Establish the monorepo structure with Go backend and Next.js frontend workspaces.
- Set up local orchestration (Docker Compose) for Postgres, Redis, API, worker, and frontend.
- Implement continuous integration (linting, unit tests) and baseline observability (structured logging, health checks).

**Key Workstreams**

- **Platform & Tooling**
  - Initialize Turborepo with Go and Next.js packages; configure PNPM workspaces and Go modules.
  - Share schema definitions (JSON Schema or protobuf) between backend and frontend packages.
  - Author Dockerfiles for API, worker, frontend, Redis, and Postgres plus `docker-compose.yml` profiles for CPU-only and GPU-enabled environments.
  - Script native startup commands for users running outside containers.
- **Observability & Testing Foundations**
  - Configure GitHub Actions to run `golangci-lint`, Go unit tests, ESLint, Jest, and Docker image builds with published artifacts.
  - Adopt `uber-go/zap` for structured logging and expose baseline health checks.

**Exit Criteria**

- Monorepo scaffolding merged with automated lint/test pipelines.
- Local development stack (Docker Compose and native scripts) successfully spins up all core services.
- Basic observability (health checks, structured logs) operational in all services.

## Phase 2: MVP Translation Pipeline (Weeks 4-8)

> **Progress checkpoint (2025-10-25):** In-memory session creation and retrieval APIs are merged. The next pick-up point is persisting
> session metadata to Postgres and wiring the handlers to enqueue work for the ingestion pipeline.

**Objectives**

- Deliver end-to-end ingestion → audio normalization → ASR → translation → subtitle output.
- Provide REST/WebSocket APIs for session control and live status updates.
- Build a minimal UI for stream setup, translation language selection, and live subtitle display.

**Key Workstreams**

- **Stream Ingestion & Media Pipeline**
  - Implement adapters for HLS, DASH, and RTMP under `services/ingestion/`, exposing a `StreamSource` interface with buffering and retry telemetry.
  - Wrap FFmpeg in `services/media/` to normalize audio to 16 kHz mono PCM chunks and enqueue them into Redis with timestamp tracking.
  - Use `hibiken/asynq` workers to orchestrate ASR → translation tasks, persisting session metadata in Postgres and caching transient state in Redis.
- **AI Services**
  - Evaluate Whisper variants vs. alternatives; document selection in `docs/asr-selection.md` and implement GPU-aware loading with CPU fallbacks.
  - Package MarianMT/Bergamot models per language pair with configurable latency vs. accuracy presets.
- **Output Generation & Delivery**
  - Generate SRT/VTT artifacts in `services/output/` and expose subtitle buffers via WebSocket APIs.
- **Frontend Integration**
  - Create Next.js pages/components for stream configuration, language selection, and live subtitle dashboards.
- **Session Control APIs**
  - ✅ `POST /sessions` validates payloads against shared schema expectations and registers sessions in memory for MVP coordination.
  - ✅ `GET /sessions/{id}` retrieves stored session configurations for downstream pipeline stages.
  - ⏭️ Persist sessions to Postgres and emit ingestion jobs so the worker can start pulling media for translation.
  - ⏭️ Add WebSocket session status updates surfaced from Redis-backed worker progress events.

**Exit Criteria**

- Demonstrable live session translating audio to subtitles with acceptable latency targets.
- Operator UI controlling session lifecycle and language selection backed by authenticated REST/WebSocket APIs.
- Integration tests covering ingestion through subtitle delivery pass reliably in CI.

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

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| GPU-dependent models exceed local hardware budgets | Pipeline stalls or quality degradation | Provide tiered model profiles (CPU-only, GPU-accelerated) and auto-detect capabilities. |
| Stream format variability causes ingestion failures | Translation sessions fail to start | Maintain pluggable adapters with comprehensive integration tests and fallback buffering strategies. |
| Latency creep across pipeline stages | User experience suffers | Instrument each stage with metrics, enable adaptive buffering, and iterate on model/queue tuning. |
| Licensing uncertainties for models/components | Deployment delays | Track licenses in `docs/licenses/` and obtain approvals early; prefer permissive models. |
| Casting interoperability issues | Inconsistent playback experience | Implement robust device discovery, provide manual fallback instructions, and gather telemetry per device type. |

---

## Phase Exit Deliverables Checklist

### Phase 1

- [ ] Turborepo monorepo with backend/frontend packages and shared schemas.
- [ ] Docker Compose stack with Postgres, Redis, API, worker, and Next.js services plus native start scripts.
- [ ] CI pipelines executing linting and unit tests with artifacts published.

### Phase 2

- [ ] Ingestion adapters with automated tests for HLS and RTMP sample streams.
- [ ] Audio normalization service producing timestamped PCM chunks via Redis queues.
- [ ] ASR + translation services with documented model selection and cached translations.
- [ ] Subtitle generator delivering synchronized SRT/VTT outputs via WebSocket APIs.
- [ ] Minimal Next.js UI for session setup, language controls, and live subtitles.

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
