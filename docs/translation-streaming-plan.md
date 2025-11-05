# Streamlation Baseline Plan

*Goal*: Build an application that ingests any web-based video or audio stream, transcribes and translates spoken content into a user-selected language using AI, and serves the translated stream back to the user in near real-time.

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
- ✅ HLS and RTMP ingestion adapters live in `packages/go/backend/ingestion`, exposing a shared `StreamSource` interface with buffering, reconnect backoff, and metrics instrumentation backed by unit tests for playlist churn and RTMP framing.
- ✅ The dedicated ingestion worker (`apps/worker/cmd/ingestion`) exercises these adapters during session warm-up to validate availability before the pipeline advances.
- ✅ File-based ingestion now streams local media via `packages/go/backend/ingestion/file.go`, allowing warm-up checks against static assets in development.
- ⏳ DASH and WebRTC sources remain unimplemented; add-ons should extend the same interface once normalization is ready.

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
- ⏳ Audio normalization is not yet implemented; the pipeline still emits stubbed stages after ingestion warm-up, so FFmpeg integration and chunk ledgers remain a priority.

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
- ⏳ No ASR or translation services have landed; the worker continues to emit sequential stub events while the ingestion layer matures.

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
- ⏳ Subtitle generation and dubbed audio outputs are outstanding pending real ASR/translation data.

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
- ✅ The Go API delivers session CRUD endpoints and WebSocket status streams, and the Next.js dashboard (`apps/web`) consumes those feeds for live monitoring.
- ⏳ Real-time delivery of translated subtitles/audio awaits downstream pipeline integration.

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
- ✅ Worker orchestration now uses a bounded goroutine pool (`apps/worker/cmd/worker`) and a separate ingestion warm-up service, improving concurrency fundamentals.
- ⏳ Observability still relies on basic structured logs; metrics, tracing, and alerting hooks are pending.

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
- ✅ Operators can register sessions and monitor live status via the existing Next.js dashboard, though accessibility and customization work remains.

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
- ⏳ Authentication and compliance documentation have not yet been implemented beyond baseline local accounts in planning documents.

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
- ✅ Implementation and architectural plans (`docs/implementation-plan.md`, `docs/final-architectural-plan.md`) track phased delivery, but roadmap/milestone docs referenced here are still to be authored.

---

This plan serves as a flexible foundation—we can adjust priorities or add detail as requirements evolve.
