# Streamlation Baseline Plan

*Goal*: Build an application that ingests any web-based video or audio stream, transcribes and translates spoken content into a user-selected language using AI, and serves the translated stream back to the user in near real-time.

---

## Overall Progress Summary

| Phase | Component | Status | Completion |
|-------|-----------|--------|------------|
| 1 | Foundation Infrastructure | Done | 100% |
| 2 | Stream Ingestion | Partial | 75% (HLS/RTMP/File done, DASH/WebRTC missing) |
| 2 | Audio Normalization | Not started | 0% |
| 2 | ASR Service | Stubbed | 5% |
| 2 | Translation Service | Stubbed | 5% |
| 2 | Output Generation | Not started | 0% |
| 3 | TTS Dubbing | Not started | 0% |
| 3 | Casting | Not started | 0% |
| 3 | Authentication | Not started | 0% |
| 4 | Observability | Basic | 10% |
| 4 | Compliance | Not started | 0% |

---

## Retail Readiness Verdict

Current progress does **not** meet first retail release expectations. Only ingestion scaffolding and operator monitoring are in place; the translation, normalization, subtitle/audio outputs, security controls, compliance documentation, and production-grade observability are outstanding. Retail positioning should be deferred until these foundational capabilities ship and pass end-to-end validation.

---

## 1. Capture & Ingest Streams
Identify target stream formats (HLS/DASH/RTMP, WebRTC, direct audio URLs) and design an ingestion service that can reliably pull and buffer live or on-demand streams. Include basic error handling for stream interruptions and format mismatches.

:::task-stub{title="Implement resilient stream ingestion layer"}
1. Survey typical web stream formats (HLS, DASH, RTMP, WebRTC) and document constraints.
2. Prototype ingestion adapters under `services/ingestion/` with a shared interface (e.g., `StreamSource` class) for pulling media chunks.
3. Add buffering and retry logic around stream reads; log interruptions and reconnections.
4. Create unit tests covering at least HLS and RTMP sample streams.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| HLS adapter | Done | `packages/go/backend/ingestion/hls.go` |
| RTMP adapter | Done | `packages/go/backend/ingestion/rtmp.go` |
| File adapter | Done | `packages/go/backend/ingestion/file.go` |
| Stream source interface | Done | `packages/go/backend/ingestion/source.go` |
| Ingestion worker | Done | `apps/worker/cmd/ingestion/` |
| DASH adapter | Not started | `packages/go/backend/ingestion/dash.go` (to create) |
| WebRTC adapter | Not started | `packages/go/backend/ingestion/webrtc.go` (to create) |

**Completion: 75%**

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Define DASH adapter | `packages/go/backend/ingestion/dash.go` | Medium |
| Define WebRTC adapter | `packages/go/backend/ingestion/webrtc.go` | Medium |
| DASH/WebRTC acceptance tests | `packages/go/backend/ingestion/dash_test.go`, `webrtc_test.go` | Medium |
| Document retry/backoff parameters | `docs/stream-ingestion.md` | Low |
| Prometheus metrics for buffer depth | `packages/go/backend/ingestion/metrics.go` | Medium |

---

## 2. Audio Extraction & Normalization
Extract audio tracks from incoming streams, convert them to a consistent codec/sample rate, and handle both video+audio and audio-only inputs.

:::task-stub{title="Normalize audio pipeline from diverse streams"}
1. Use FFmpeg bindings under `services/media/` to strip audio from video streams and normalize to a target sample rate (e.g., 16 kHz, mono).
2. Implement a queueing mechanism (e.g., Redis, Kafka, or in-memory prototype) to pass normalized audio chunks to downstream services.
3. Add telemetry around audio frame timestamps to keep translation aligned.
4. Provide integration tests with short sample streams verifying consistent audio output.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| FFmpeg wrapper | Not started | `packages/go/backend/media/ffmpeg.go` (to create) |
| Normalizer interface | Not started | `packages/go/backend/media/normalizer.go` (to create) |
| FFmpeg normalizer | Not started | `packages/go/backend/media/ffmpeg_normalizer.go` (to create) |
| Stub normalizer | Not started | `packages/go/backend/media/stub_normalizer.go` (to create) |
| Chunk ledger | Not started | `packages/go/backend/media/ledger.go` (to create) |
| Audio chunk schema | Not started | `packages/schemas/audio-chunk.schema.json` (to create) |

**Completion: 0%**

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Define Normalizer interface | `packages/go/backend/media/normalizer.go` | Critical |
| Implement FFmpeg wrapper | `packages/go/backend/media/ffmpeg.go` | Critical |
| Create FFmpeg normalizer | `packages/go/backend/media/ffmpeg_normalizer.go` | Critical |
| Create stub normalizer for testing | `packages/go/backend/media/stub_normalizer.go` | High |
| Implement chunk ledger in Postgres | `packages/go/backend/media/ledger.go`, migration in `packages/go/backend/postgres/migrations/` | High |
| Add test data for normalization | `packages/go/backend/media/testdata/` | Medium |
| Document audio normalization spec | `docs/audio-normalization.md` | Low |
| Add waveform analytics (RMS/peak) | `packages/go/backend/media/analytics.go` | Medium |

---

## 3. Speech Recognition & Translation
Select an ASR model (e.g., Whisper, DeepSpeech, cloud APIs) and a translation model/service capable of real-time or near-real-time performance. Support configurable target languages.

:::task-stub{title="Build ASR→translation microservice"}
1. Evaluate candidate ASR APIs/models for latency vs. accuracy; document findings in `docs/asr-selection.md`.
2. Implement an ASR service (e.g., `services/transcribe/`) that consumes normalized audio chunks and outputs timestamped transcripts.
3. Chain translation (e.g., OpenAI, Marian, or custom model) to convert transcripts into user-selected languages; maintain timestamps for subtitle alignment.
4. Cache recent translations to mitigate repeated phrases.
5. Cover service logic with unit tests using prerecorded multilingual samples.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Recognizer interface | Not started | `packages/go/backend/asr/recognizer.go` (to create) |
| Whisper recognizer | Not started | `packages/go/backend/asr/whisper_recognizer.go` (to create) |
| Stub recognizer | Not started | `packages/go/backend/asr/stub_recognizer.go` (to create) |
| Translator interface | Not started | `packages/go/backend/translation/translator.go` (to create) |
| MarianMT translator | Not started | `packages/go/backend/translation/marian_translator.go` (to create) |
| Stub translator | Not started | `packages/go/backend/translation/stub_translator.go` (to create) |
| Translation cache | Not started | `packages/go/backend/translation/cache.go` (to create) |
| Pipeline stub | Exists (fake events) | `packages/go/backend/pipeline/pipeline.go:17-73` |

**Completion: 5%** (only stub pipeline emitting fake events)

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Define Recognizer interface | `packages/go/backend/asr/recognizer.go` | Critical |
| Implement Whisper wrapper | `packages/go/backend/asr/whisper_recognizer.go` | Critical |
| Create stub recognizer for testing | `packages/go/backend/asr/stub_recognizer.go` | High |
| Add ASR test data | `packages/go/backend/asr/testdata/transcripts/` | High |
| Define Translator interface | `packages/go/backend/translation/translator.go` | Critical |
| Implement MarianMT wrapper | `packages/go/backend/translation/marian_translator.go` | Critical |
| Create stub translator for testing | `packages/go/backend/translation/stub_translator.go` | High |
| Add translation dictionaries | `packages/go/backend/translation/testdata/dictionaries/` | High |
| Implement translation cache | `packages/go/backend/translation/cache.go` | Medium |
| Finalize model selection memo | `docs/asr-selection.md` | Medium |
| Wire ASR to pipeline | Update `packages/go/backend/pipeline/pipeline.go` | Critical |

---

## 4. Output Generation (Text & Audio)
Provide multiple output modalities: translated subtitles (SRT/VTT) and optional synthesized audio (TTS) to re-dub the stream.

:::task-stub{title="Produce translated subtitles and optional dubbed audio"}
1. Create subtitle generator in `services/output/` that merges translated text with timestamps into SRT/VTT formats.
2. Integrate a TTS engine (cloud or open source) to synthesize translated audio; ensure synchronization with original stream timing.
3. Offer APIs to fetch translated subtitles and/or TTS audio segments.
4. Write end-to-end tests confirming subtitle timing and TTS audio length alignment.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| SubtitleGenerator interface | Not started | `packages/go/backend/output/generator.go` (to create) |
| SRT generator | Not started | `packages/go/backend/output/srt.go` (to create) |
| VTT generator | Not started | `packages/go/backend/output/vtt.go` (to create) |
| Stub generator | Not started | `packages/go/backend/output/stub_generator.go` (to create) |
| Synthesizer interface | Not started | `packages/go/backend/tts/synthesizer.go` (to create) |
| Coqui TTS | Not started | `packages/go/backend/tts/coqui.go` (to create) |
| Bark TTS | Not started | `packages/go/backend/tts/bark.go` (to create) |
| Stub synthesizer | Not started | `packages/go/backend/tts/stub_synthesizer.go` (to create) |
| Subtitle schema | Not started | `packages/schemas/subtitle.schema.json` (to create) |

**Completion: 0%**

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Define SubtitleGenerator interface | `packages/go/backend/output/generator.go` | Critical |
| Implement SRT generator | `packages/go/backend/output/srt.go` | Critical |
| Implement VTT generator | `packages/go/backend/output/vtt.go` | Critical |
| Create stub generator for testing | `packages/go/backend/output/stub_generator.go` | High |
| Define subtitle schema | `packages/schemas/subtitle.schema.json` | High |
| Add subtitle API endpoint | `apps/api/cmd/server/subtitles.go` | High |
| Define Synthesizer interface | `packages/go/backend/tts/synthesizer.go` | Medium |
| Evaluate Coqui vs. Bark | `docs/tts-selection.md` | Medium |
| Implement Coqui TTS wrapper | `packages/go/backend/tts/coqui.go` | Medium |
| Create stub synthesizer | `packages/go/backend/tts/stub_synthesizer.go` | Medium |
| Add TTS test audio | `packages/go/backend/tts/testdata/voices/` | Medium |
| Build diff-stream publisher | `packages/go/backend/output/stream.go` | High |

---

## 5. Real-Time Delivery Layer
Design a delivery mechanism (web client or API) that serves the translated outputs, handles live updates, and syncs them with the original stream.

:::task-stub{title="Implement delivery API and real-time sync"}
1. Build REST/WebSocket endpoints in `api/` to distribute translated subtitles and TTS audio chunks.
2. Implement client-side synchronization logic (e.g., React/Next.js front end) ensuring subtitles/audio stay aligned with the original stream’s playback position.
3. Add buffering strategies to cope with translation latency.
4. Create integration tests with mocked clients verifying real-time updates.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Session CRUD API | Done | `apps/api/cmd/server/session.go` |
| WebSocket status streaming | Done | `apps/api/cmd/server/status.go` |
| Next.js dashboard | Done | `apps/web/app/page.tsx` |
| Subtitle live stream endpoint | Not started | `apps/api/cmd/server/subtitles.go` (to create) |
| Client-side buffering | Not started | `apps/web/app/hooks/useSubtitleSync.ts` (to create) |
| MediaSource integration | Not started | `apps/web/app/components/VideoPlayer.tsx` (to create) |

**Completion: 40%** (basic status streaming works, subtitle delivery pending)

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Add subtitle WebSocket endpoint | `apps/api/cmd/server/subtitles.go` | Critical |
| Build useSubtitleSync hook | `apps/web/app/hooks/useSubtitleSync.ts` | High |
| Implement VideoPlayer component | `apps/web/app/components/VideoPlayer.tsx` | High |
| Add MediaSource integration | `apps/web/app/lib/mediaSource.ts` | High |
| Document sync tolerance budgets | `docs/delivery-sync.md` | Medium |
| Add dubbed audio streaming | `apps/api/cmd/server/audio.go` | Medium |

---

## 6. Orchestration, Scalability & Monitoring
Plan for scalable deployment, fault tolerance, logging, and observability.

:::task-stub{title="Establish deployment and observability foundations"}
1. Define containerization strategy (Dockerfiles) for each service; compose them via Docker Compose or Kubernetes manifests.
2. Implement centralized logging and metrics (Prometheus/Grafana or cloud equivalents) for ingestion, ASR, translation, and delivery services.
3. Add alerting rules for common failure modes (stream drop, ASR lag, translation errors).
4. Document deployment workflow in `docs/deployment.md`.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Worker goroutine pool | Done | `apps/worker/cmd/worker/main.go` |
| Ingestion warm-up service | Done | `apps/worker/cmd/ingestion/` |
| Structured logging (zap) | Done | `third_party/go.uber.org/zap/` |
| Docker Compose stack | Done | `docker-compose.yml` |
| CI pipeline | Done | `.github/workflows/ci.yml` |
| OpenTelemetry tracing | Not started | `packages/go/backend/telemetry/tracer.go` (to create) |
| Prometheus metrics | Not started | `packages/go/backend/telemetry/metrics.go` (to create) |
| Alerting rules | Not started | `observability/alerts/` (to create) |
| Grafana dashboards | Not started | `observability/dashboards/` (to create) |

**Completion: 30%** (basic orchestration done, observability pending)

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Create telemetry package | `packages/go/backend/telemetry/` | High |
| Add OpenTelemetry tracing | `packages/go/backend/telemetry/tracer.go` | High |
| Define Prometheus metrics | `packages/go/backend/telemetry/metrics.go` | High |
| Instrument API handlers | `apps/api/cmd/server/*.go` | High |
| Instrument worker stages | `apps/worker/cmd/worker/main.go` | High |
| Create Grafana dashboards | `observability/dashboards/*.json` | Medium |
| Define alerting rules | `observability/alerts/*.yaml` | Medium |
| Create deployment playbook | `docs/deployment.md` | Medium |
| Draft K8s Helm chart | `deploy/k8s/helm/` | Low |

---

## 7. UX & Accessibility Considerations
Ensure the interface supports language selection, stream configuration, and accessibility (subtitle customization, audio descriptions).

:::task-stub{title="Design accessible front-end controls"}
1. Create UI mockups/wireframes (store in `design/`) covering stream URL input, language selection, subtitle styling, and audio toggles.
2. Implement accessible components with keyboard navigation and ARIA labels.
3. Provide user preference persistence (local storage or backend profile).
4. Conduct usability testing sessions and summarize findings.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Session registration form | Done | `apps/web/app/page.tsx` |
| Live status monitoring | Done | `apps/web/app/page.tsx` |
| Session list display | Done | `apps/web/app/page.tsx` |
| Language selection UI | Basic | `apps/web/app/page.tsx` |
| Subtitle customization | Not started | `apps/web/app/components/SubtitleSettings.tsx` (to create) |
| Casting controls | Not started | `apps/web/app/components/CastingControls.tsx` (to create) |
| Accessibility (ARIA) | Partial | Needs audit |
| Keyboard navigation | Partial | Needs audit |
| High-contrast themes | Not started | `apps/web/app/globals.css` |
| User preferences API | Not started | `apps/api/cmd/server/preferences.go` (to create) |

**Completion: 40%** (basic UI done, polish and accessibility pending)

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Create SubtitleSettings component | `apps/web/app/components/SubtitleSettings.tsx` | Medium |
| Create CastingControls component | `apps/web/app/components/CastingControls.tsx` | Medium |
| Add high-contrast theme | `apps/web/app/globals.css`, `apps/web/tailwind.config.ts` | Medium |
| Implement keyboard navigation | `apps/web/app/components/*.tsx` | Medium |
| Add ARIA labels | `apps/web/app/components/*.tsx` | Medium |
| Create user preferences API | `apps/api/cmd/server/preferences.go` | Low |
| Document accessibility audit | `docs/accessibility-audit.md` | Low |
| Create UI spec document | `design/ux-streamlation.fig` or `docs/ui-spec.md` | Low |

---

## 8. Security & Compliance
Address user data privacy, API keys, and potential licensing requirements for stream content.

:::task-stub{title="Harden security and compliance posture"}
1. Define authentication/authorization strategy for accessing translation services.
2. Securely store API keys/secrets using environment variables or secret managers.
3. Draft a compliance checklist (e.g., GDPR considerations) in `docs/compliance.md`.
4. Implement logging/auditing around stream access and translation requests.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Authentication strategy | Planning only | See `docs/final-architectural-plan.md` |
| Go JWT issuer | Not started | `apps/api/cmd/server/auth.go` (to create) |
| Auth middleware | Not started | `apps/api/cmd/server/middleware/auth.go` (to create) |
| NextAuth integration | Not started | `apps/web/app/api/auth/[...nextauth]/route.ts` (to create) |
| Audit logging | Not started | `packages/go/backend/audit/logger.go` (to create) |
| Secrets management | Not started | Vault/1Password integration |
| Compliance documentation | Not started | `docs/compliance.md` (to create) |
| Data retention API | Not started | `packages/go/backend/retention/purge.go` (to create) |

**Completion: 0%**

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Draft security architecture | `docs/security-architecture.md` | High |
| Implement Go JWT issuer | `apps/api/cmd/server/auth.go` | High |
| Create auth middleware | `apps/api/cmd/server/middleware/auth.go` | High |
| Set up NextAuth | `apps/web/app/api/auth/[...nextauth]/route.ts` | High |
| Implement audit logging | `packages/go/backend/audit/logger.go` | Medium |
| Add audit events table | `packages/go/backend/postgres/migrations/audit.sql` | Medium |
| Create compliance checklist | `docs/compliance.md` | Medium |
| Implement data retention purge | `packages/go/backend/retention/purge.go` | Medium |
| Document GDPR considerations | `docs/compliance.md` | Medium |
| Track third-party licenses | `docs/licenses/` | Low |

---

## 9. MVP Roadmap & Milestones
Outline phased delivery: MVP (single stream support, limited languages), beta (multi-stream, improved latency), production (scaling, monitoring).

:::task-stub{title="Draft phased roadmap"}
1. Enumerate MVP, beta, and production feature sets in `docs/roadmap.md`.
2. Map dependencies between task-stubs; estimate timelines.
3. Highlight research spikes (e.g., ASR evaluation) vs. engineering tasks.
4. Review roadmap with stakeholders and iterate.
:::

### Progress

| Item | Status | Location |
|------|--------|----------|
| Implementation plan | Done | `docs/implementation-plan.md` |
| Architectural plan | Done | `docs/final-architectural-plan.md` |
| Baseline plan (this doc) | Done | `docs/translation-streaming-plan.md` |
| Formal roadmap doc | Not started | `docs/roadmap.md` (to create) |
| Dependency mapping | Partial | See `docs/implementation-plan.md` |

**Completion: 60%** (planning docs exist but formal roadmap pending)

**Next Actions**

| Action | Implementation Location | Priority |
|--------|------------------------|----------|
| Author formal roadmap | `docs/roadmap.md` | Low |
| Map critical path dependencies | `docs/implementation-plan.md` (update) | Low |
| Create milestone tracking | GitHub Projects or `docs/milestones.md` | Low |

---

## Summary: Critical Path to MVP

The following items must be completed in order to deliver a functioning MVP:

| # | Component | Implementation Location | Blocked By |
|---|-----------|------------------------|------------|
| 1 | Audio Normalizer | `packages/go/backend/media/normalize.go` | None |
| 2 | ASR Service | `packages/go/backend/asr/whisper_recognizer.go` | #1 |
| 3 | Translation Service | `packages/go/backend/translation/marian_translator.go` | #2 |
| 4 | Subtitle Generator | `packages/go/backend/output/subtitle.go` | #3 |
| 5 | Wire Pipeline | `packages/go/backend/pipeline/production_runner.go` | #1, #2, #3, #4 |
| 6 | Subtitle API | `apps/api/cmd/server/subtitles.go` | #4 |
| 7 | Frontend Player | `apps/web/app/components/VideoPlayer.tsx` | #6 |

For testability without real AI models, implement stub versions first:
- `packages/go/backend/media/stub_normalizer.go`
- `packages/go/backend/asr/stub_recognizer.go`
- `packages/go/backend/translation/stub_translator.go`
- `packages/go/backend/output/stub_generator.go`
- `packages/go/backend/pipeline/testable_runner.go`

See `docs/implementation-plan.md` → "Scaffold Implementation for Testability" for detailed instructions.

---

This plan serves as a flexible foundation—we can adjust priorities or add detail as requirements evolve.
