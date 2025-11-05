# Final Architectural Plan for Streamlation

## Guiding Principles
- Deliver low-latency, high-fidelity voice translation layered onto arbitrary streaming sources while preserving the original speaker’s voice.
- Favor components that run entirely on a local machine, minimizing external dependencies.
- Maintain a clean separation between the media/AI processing pipeline, backend services, and frontend UI.

---

## Backend Architecture (Go)

### Core Stack
- **Language & Runtime:** Go (Golang) for performance, concurrency, and single-binary distribution.
- **HTTP Framework:** Go Fiber for REST endpoints and middleware; native WebSocket handling for realtime updates.
- **API Contract:** REST endpoints with OpenAPI documentation via `swaggo/swag`; WebSocket channels for session status, buffer levels, and casting controls.

### Streaming & Casting
- **Stream Ingestion:** Wrap `ffmpeg` or `gstreamer` to accept HLS, DASH, MP4, or direct stream URLs (including YouTube).
- **Casting Integrations:** Provide APIs that bridge to Chromecast (CAF receiver + mDNS discovery) and AirPlay (via open-source RAOP/AirPlay libraries). Default to open protocols, with optional SDK support when credentials are supplied.

### AI Processing Pipeline
1. **Audio Extraction:** FFmpeg isolates audio with configurable pre-buffer to absorb translation latency.
2. **Speech Recognition:** Locally run Whisper (via `whisper.cpp`) with GPU acceleration when available; fall back to CPU with smaller models.
3. **Machine Translation:** MarianMT or Bergamot models executed through ONNX Runtime or native binaries; language packs cached locally.
4. **Voice Cloning & Synthesis:** Coqui TTS or Bark-based models fine-tuned to match source speaker timbre using adaptive voice cloning; fallback to high-quality multi-speaker TTS if cloning resources are insufficient.
5. **Pipeline Orchestration:** `hibiken/asynq` workers coordinate STT → MT → TTS tasks, using Redis queues for buffering and retries.
6. **Output Mixing:** Composite translated audio with original video stream or supply standalone audio track selectable by the frontend player.

### Data & State Management
- **Primary Database:** PostgreSQL (Dockerized locally) storing users, OAuth sessions, translation session metadata, casting devices, and model inventories.
- **Cache/Buffers:** Redis for job queues, rate limiting, WebSocket session state, and short-lived translated audio buffers.
- **Configuration:** Viper-driven environment settings with Dotenv defaults packaged for offline operation.

### Authentication & Security
- **Identity Providers:** Email/password plus OAuth 2.0/OpenID Connect (Google, Apple, GitHub). Store hashed credentials locally.
- **Tokens:** JWT (access + refresh) with configurable lifetimes; device tokens for casting hardware pairing.
- **Local Privacy Controls:** User-owned transcripts and audio buffers with configurable retention policies; no cloud uploads by default.
- **Hardening:** Input validation on stream URLs, ffmpeg sandboxing (restrict protocols), and optional rate limiting even in local mode.

### Observability & Testing
- **Logging:** Structured logs with `uber-go/zap`; adjustable verbosity.
- **Metrics:** Prometheus-compatible endpoint capturing pipeline latency, queue depth, translation accuracy metrics, casting success rates.
- **Testing:** Go `testing` + table-driven tests; integration tests via `testcontainers-go` for Postgres/Redis; scenario tests mocking AI pipeline stages.

---

## Frontend Architecture (Next.js + TypeScript)

### Core Stack
- **Framework:** Next.js (App Router) with TypeScript for SSR/SSG flexibility and shared typing.
- **Styling & UI:** Tailwind CSS with Radix UI components; theming for light/dark modes and accessibility basics.
- **State Management:** TanStack Query for server state (session progress, available models); Zustand for local UI preferences (language, voice selection).

### Media Playback & Controls
- **Player:** Video.js with HLS.js plugin, enabling arbitrary stream URLs and custom audio track overlays.
- **Translation Controls:** React Hook Form + Zod powering a control panel to select target language, voice profile, latency tolerance, and casting target.
- **Realtime Feedback:** WebSocket-fed dashboard showing detected language, buffer depth, estimated latency, and transcription snippets.
- **Casting UX:** Chromecast and AirPlay buttons leveraging native browser APIs/SDKs; fallback instructions for manual casting if unsupported.

### Authentication & Session UX
- **Auth Layer:** NextAuth.js configured for local accounts and OAuth providers, exchanging tokens with the Go backend.
- **Offline Support:** Persistent sessions and cached user settings so the app operates without internet access after initial setup.

### Testing & Quality
- **Unit/Component Tests:** Jest + React Testing Library.
- **End-to-End:** Playwright covering login, stream setup, translation toggles, casting initiation, and failure handling (e.g., unsupported streams).

---

## AI Model Packaging & Distribution
- **Model Bundles:** Provide installers/download scripts for Whisper, MarianMT, and Coqui/Bark models per language pair, respecting permissive licenses (MIT, Apache 2.0, MPL).
- **Hardware Profiles:** Offer presets (e.g., “CPU-only”, “GPU-enabled”) adjusting model sizes and buffer defaults; document minimum specs (e.g., 8-core CPU, 16 GB RAM for CPU mode).
- **Updates:** Modular model directory with semantic versioning; background job checks for updates when online, with manual import/export for offline environments.
- **Voice Enrollment:** Optional feature to capture short voice samples to improve cloning fidelity; encrypted local storage of embeddings.

---

## Local Deployment & Tooling
- **Monorepo:** Turborepo managing Go backend (separate module) and Next.js frontend, with shared DTO schemas (JSON Schema or protobuf) consumed on both sides.
- **Package Management:** PNPM workspaces for frontend; Go modules for backend.
- **Local Orchestration:** Docker Compose stack (API, worker, Redis, Postgres, Next.js) plus scripts for native execution without containers.
- **Build & Release:** GitHub Actions running lint (`golangci-lint`, ESLint), tests, and Docker builds to ensure reproducible artifacts before packaging installers for macOS/Windows/Linux.

---

## Security, Privacy, and Compliance
- **Local-First Privacy:** Default configuration avoids any outbound network calls except optional OAuth verification and model updates.
- **Data Governance:** Provide UI and CLI tools to purge transcripts, buffers, and cached credentials.
- **Content Safeguards:** Optional profanity/abuse filters during translation pipeline; configurable by user.
- **Licensing Compliance:** Track third-party model licenses, prompt users to acknowledge on installation, and provide attribution within the UI.

---

## Assumptions & Resolved Decisions
1. **Model Licensing:** Favor open-source models under Apache/MIT/Mozilla licenses (Whisper, MarianMT, Coqui); document obligations in installer.
2. **Voice Fidelity:** Aim for high likeness using adaptive voice cloning; provide fallback multi-speaker TTS if hardware cannot sustain cloning.
3. **Casting Support:** Bundle open-protocol implementations by default; allow users to supply credentials/API keys for official Chromecast/AirPlay SDK features.
4. **Hardware Baseline:** Publish recommended specs (e.g., modern 6+ core CPU, 16 GB RAM, optional NVIDIA GPU with CUDA) and auto-tune pipeline to detected hardware.
5. **Offline Updates:** Provide manual model/software update packages (.zip installers); when online, optional background checker with user consent.

---

## Implementation Progress Snapshot (Current Phase)
- **Ingestion Adapters:** `packages/go/backend/ingestion` now houses HLS, RTMP, and file-based sources with jitter buffering, reconnect logic, metrics counters, and unit coverage for playlist churn, framing edge cases, and local media replay.
- **Worker Concurrency:** `apps/worker/cmd/worker` fans ingestion jobs across a bounded goroutine pool while `apps/worker/cmd/ingestion` performs stream warm-up using the new adapters before the pipeline stub emits stage events.
- **Operator Surface:** The Next.js dashboard (`apps/web`) and Go API continue to provide session CRUD plus WebSocket status feeds, enabling manual verification of ingestion progress while downstream media stages remain stubbed.

---

## Roadmap Highlights
1. **MVP Phase**
   - Stream ingestion with translation pipeline (English ↔ target language subset).
   - Local auth + OAuth login, session dashboard, and basic casting.
   - CPU-friendly model defaults with optional GPU acceleration.

2. **Enhancement Phase**
   - Expanded language packs, improved voice cloning accuracy, customization options.
   - Advanced monitoring dashboard, granular latency controls, and session recording export.
   - Installer packages and self-update workflows.

3. **Future Explorations**
   - Multi-speaker recognition for group streams.
   - Plugin interface for custom translation engines or cloud offloading.
   - Community model marketplace and sharing features.

