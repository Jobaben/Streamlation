'use client';

import Link from "next/link";
import {
  FormEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";

const API_BASE_RAW = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
const API_BASE = API_BASE_RAW.endsWith("/")
  ? API_BASE_RAW.slice(0, API_BASE_RAW.length - 1)
  : API_BASE_RAW;

const sourceTypes = ["hls", "dash", "rtmp", "file"] as const;
const modelProfiles = ["cpu-basic", "cpu-advanced", "gpu-accelerated"] as const;
const languageSuggestions = ["en", "es", "fr", "de", "it", "ja", "ko", "pt", "hi"];

type TranslationSession = {
  id: string;
  source: {
    type: string;
    uri: string;
  };
  targetLanguage: string;
  options: {
    enableDubbing: boolean;
    latencyToleranceMs: number;
    modelProfile: string;
  };
};

type SessionStatusEvent = {
  sessionId: string;
  stage: string;
  state: string;
  detail?: string;
  timestamp: string;
};

type SessionFormState = {
  id: string;
  sourceType: string;
  sourceUri: string;
  targetLanguage: string;
  enableDubbing: boolean;
  latencyToleranceMs: string;
  modelProfile: string;
};

type ConnectionState = "idle" | "connecting" | "open" | "closed" | "error";

const initialFormState: SessionFormState = {
  id: "",
  sourceType: sourceTypes[0],
  sourceUri: "",
  targetLanguage: "en",
  enableDubbing: false,
  latencyToleranceMs: "5000",
  modelProfile: modelProfiles[0]
};

function buildApiPath(path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  return `${API_BASE}${normalizedPath}`;
}

function buildWebSocketURL(path: string): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (API_BASE.startsWith("ws://") || API_BASE.startsWith("wss://")) {
    return `${API_BASE}${normalizedPath}`;
  }
  if (API_BASE.startsWith("https://")) {
    return `wss://${API_BASE.slice("https://".length)}${normalizedPath}`;
  }
  if (API_BASE.startsWith("http://")) {
    return `ws://${API_BASE.slice("http://".length)}${normalizedPath}`;
  }
  return `${API_BASE}${normalizedPath}`;
}

async function extractErrorMessage(response: Response): Promise<string> {
  try {
    const payload = await response.json();
    if (payload && typeof payload === "object" && "error" in payload) {
      const message = (payload as { error?: unknown }).error;
      if (typeof message === "string" && message.trim() !== "") {
        return message;
      }
    }
  } catch (error) {
    console.warn("Failed to parse error response", error);
  }
  return `Request failed with status ${response.status}`;
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${date.toLocaleDateString()} ${date.toLocaleTimeString()}`;
}

function connectionLabel(state: ConnectionState): string {
  switch (state) {
    case "connecting":
      return "Connecting";
    case "open":
      return "Live";
    case "closed":
      return "Closed";
    case "error":
      return "Error";
    default:
      return "Idle";
  }
}

export default function HomePage(): JSX.Element {
  const [form, setForm] = useState<SessionFormState>(initialFormState);
  const [sessions, setSessions] = useState<TranslationSession[]>([]);
  const [statusMessage, setStatusMessage] = useState<
    | {
        type: "success" | "error";
        text: string;
      }
    | null
  >(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [sessionError, setSessionError] = useState<string | null>(null);
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [statusEvents, setStatusEvents] = useState<SessionStatusEvent[]>([]);
  const [connectionState, setConnectionState] = useState<ConnectionState>("idle");
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const selectedSession = useMemo(() => {
    if (!selectedSessionId) {
      return null;
    }
    return sessions.find((session) => session.id === selectedSessionId) ?? null;
  }, [sessions, selectedSessionId]);

  const loadSessions = useCallback(async () => {
    setLoadingSessions(true);
    setSessionError(null);
    try {
      const response = await fetch(buildApiPath("/sessions?limit=50"));
      if (!response.ok) {
        throw new Error(await extractErrorMessage(response));
      }
      const data = (await response.json()) as TranslationSession[];
      setSessions(data);
      if (selectedSessionId && !data.some((session) => session.id === selectedSessionId)) {
        setSelectedSessionId(null);
      }
    } catch (error) {
      setSessionError(
        error instanceof Error ? error.message : "Failed to load sessions"
      );
    } finally {
      setLoadingSessions(false);
    }
  }, [selectedSessionId]);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  useEffect(() => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    setStatusEvents([]);
    setConnectionError(null);

    if (!selectedSessionId) {
      setConnectionState("idle");
      return;
    }

    const url = buildWebSocketURL(`/sessions/${encodeURIComponent(selectedSessionId)}/events`);
    setConnectionState("connecting");

    let isUnmounted = false;

    try {
      const socket = new WebSocket(url);
      wsRef.current = socket;

      socket.onopen = () => {
        if (!isUnmounted) {
          setConnectionState("open");
        }
      };

      socket.onclose = () => {
        if (!isUnmounted) {
          setConnectionState("closed");
        }
      };

      socket.onerror = () => {
        if (!isUnmounted) {
          setConnectionState("error");
          setConnectionError("Unable to maintain session status stream.");
        }
      };

      socket.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data) as Partial<SessionStatusEvent>;
          const normalized: SessionStatusEvent = {
            sessionId: payload.sessionId ?? selectedSessionId,
            stage: payload.stage ?? "unknown",
            state: payload.state ?? "unknown",
            detail: payload.detail ?? undefined,
            timestamp:
              payload.timestamp ?? new Date().toISOString()
          };
          setStatusEvents((prev) => [normalized, ...prev].slice(0, 50));
        } catch (error) {
          console.error("Failed to parse status event", error);
        }
      };
    } catch (error) {
      setConnectionState("error");
      setConnectionError(
        error instanceof Error
          ? error.message
          : "Failed to connect to status stream."
      );
    }

    return () => {
      isUnmounted = true;
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [selectedSessionId]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setStatusMessage(null);

    const trimmedId = form.id.trim();
    const trimmedUri = form.sourceUri.trim();
    const trimmedLanguage = form.targetLanguage.trim().toLowerCase();
    const latency = Number(form.latencyToleranceMs);

    if (trimmedId.length < 8 || trimmedId.length > 64) {
      setStatusMessage({
        type: "error",
        text: "Session ID must be between 8 and 64 characters."
      });
      return;
    }

    if (!Number.isFinite(latency) || latency < 0 || latency > 60000) {
      setStatusMessage({
        type: "error",
        text: "Latency tolerance must be between 0 and 60000 milliseconds."
      });
      return;
    }

    setIsSubmitting(true);
    try {
      const payload = {
        id: trimmedId,
        source: {
          type: form.sourceType,
          uri: trimmedUri
        },
        targetLanguage: trimmedLanguage,
        options: {
          enableDubbing: form.enableDubbing,
          latencyToleranceMs: latency,
          modelProfile: form.modelProfile
        }
      };

      const response = await fetch(buildApiPath("/sessions"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(payload)
      });

      if (!response.ok) {
        throw new Error(await extractErrorMessage(response));
      }

      const created = (await response.json()) as TranslationSession;
      setStatusMessage({
        type: "success",
        text: `Session ${created.id} persisted and ingestion queued.`
      });
      setSessions((prev) => {
        const filtered = prev.filter((session) => session.id !== created.id);
        return [created, ...filtered];
      });
      setSelectedSessionId(created.id);
      setStatusEvents([]);
      setForm((previous) => ({
        ...previous,
        id: "",
        sourceUri: ""
      }));
      void loadSessions();
    } catch (error) {
      setStatusMessage({
        type: "error",
        text: error instanceof Error ? error.message : "Failed to create session."
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleReset = () => {
    setForm(initialFormState);
    setStatusMessage(null);
  };

  return (
    <main>
      <header className="page-header">
        <h1>Streamlation session coordinator</h1>
        <p>
          Register translation sessions with the Go API, persist them to Postgres,
          and queue ingestion jobs through Redis. This UI mirrors the milestones
          called out in the implementation plan to validate the MVP pipeline end
          to end.
        </p>
        <p>
          Explore the <Link href="https://github.com/golang/go/wiki/Modules">Go
          modules guide</Link> or review the implementation plan in the docs
          directory to dive deeper into sequencing decisions.
        </p>
      </header>

      <section>
        <h2>Create a translation session</h2>
        <p>
          Provide a stream source, target language, and tuning options. Successful
          submissions persist to Postgres and emit an ingestion job for the worker
          queue.
        </p>
        {statusMessage ? (
          <div className={`status-message ${statusMessage.type}`}>
            {statusMessage.text}
          </div>
        ) : null}
        <form onSubmit={handleSubmit}>
          <fieldset>
            <legend>Identity</legend>
            <label htmlFor="session-id">
              Session ID
              <input
                id="session-id"
                name="sessionId"
                value={form.id}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    id: event.target.value
                  }))
                }
                pattern="^[A-Za-z0-9_-]{8,64}$"
                required
                placeholder="creator-live-01"
                autoComplete="off"
              />
              <span className="input-help">
                8-64 characters. Letters, numbers, dash, and underscore are
                supported.
              </span>
            </label>
            <label htmlFor="target-language">
              Target language
              <input
                id="target-language"
                name="targetLanguage"
                value={form.targetLanguage}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    targetLanguage: event.target.value.toLowerCase()
                  }))
                }
                list="language-options"
                pattern="^[a-z]{2}$"
                required
                placeholder="en"
                autoComplete="off"
              />
              <span className="input-help">Two-letter ISO 639-1 code.</span>
              <datalist id="language-options">
                {languageSuggestions.map((code) => (
                  <option key={code} value={code} />
                ))}
              </datalist>
            </label>
          </fieldset>

          <fieldset>
            <legend>Source</legend>
            <label htmlFor="source-type">
              Source type
              <select
                id="source-type"
                name="sourceType"
                value={form.sourceType}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    sourceType: event.target.value
                  }))
                }
              >
                {sourceTypes.map((value) => (
                  <option key={value} value={value}>
                    {value.toUpperCase()}
                  </option>
                ))}
              </select>
            </label>
            <label htmlFor="source-uri">
              Source URI
              <input
                id="source-uri"
                name="sourceUri"
                type="url"
                value={form.sourceUri}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    sourceUri: event.target.value
                  }))
                }
                required
                placeholder="https://cdn.example.com/stream.m3u8"
              />
            </label>
          </fieldset>

          <fieldset>
            <legend>Options</legend>
            <label className="checkbox" htmlFor="enable-dubbing">
              <input
                id="enable-dubbing"
                name="enableDubbing"
                type="checkbox"
                checked={form.enableDubbing}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    enableDubbing: event.target.checked
                  }))
                }
              />
              Enable AI dubbing for this session
            </label>
            <label htmlFor="latency-tolerance">
              Latency tolerance (milliseconds)
              <input
                id="latency-tolerance"
                name="latencyTolerance"
                type="number"
                min={0}
                max={60000}
                step={100}
                value={form.latencyToleranceMs}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    latencyToleranceMs: event.target.value
                  }))
                }
              />
            </label>
            <label htmlFor="model-profile">
              Model profile
              <select
                id="model-profile"
                name="modelProfile"
                value={form.modelProfile}
                onChange={(event) =>
                  setForm((previous) => ({
                    ...previous,
                    modelProfile: event.target.value
                  }))
                }
              >
                {modelProfiles.map((profile) => (
                  <option key={profile} value={profile}>
                    {profile}
                  </option>
                ))}
              </select>
            </label>
          </fieldset>
          <div className="button-row">
            <button type="submit" disabled={isSubmitting}>
              {isSubmitting ? "Registering session…" : "Register session"}
            </button>
            <button
              type="button"
              className="secondary-button"
              onClick={handleReset}
              disabled={isSubmitting}
            >
              Reset form
            </button>
          </div>
        </form>
      </section>

      <section>
        <h2>Registered sessions</h2>
        <p>
          Sessions are persisted in Postgres. Refresh to pull the most recent
          activity ordered by creation time.
        </p>
        <div className="button-row">
          <button
            type="button"
            className="secondary-button"
            onClick={() => void loadSessions()}
            disabled={loadingSessions}
          >
            {loadingSessions ? "Refreshing…" : "Refresh list"}
          </button>
        </div>
        {sessionError ? (
          <div className="status-message error">{sessionError}</div>
        ) : null}
        {sessions.length === 0 ? (
          <p className="empty-state">No sessions have been registered yet.</p>
        ) : (
          <div className="table-wrapper">
            <table className="session-table">
              <thead>
                <tr>
                  <th>Session</th>
                  <th>Source</th>
                  <th>Target</th>
                  <th>Profile</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {sessions.map((session, index) => {
                  const isSelected = session.id === selectedSessionId;
                  return (
                    <tr
                      key={`${session.id}-${index}`}
                      className={isSelected ? "selected" : undefined}
                    >
                      <td>
                        <div className="session-id">{session.id}</div>
                        <div className="session-meta">
                          {session.source.type.toUpperCase()}
                        </div>
                      </td>
                      <td>
                        <div className="session-uri">{session.source.uri}</div>
                      </td>
                      <td>
                        <div className="session-meta">
                          {session.targetLanguage.toUpperCase()}
                        </div>
                        <div className="session-meta">
                          {session.options.enableDubbing ? "Dubbing" : "Subtitles"}
                        </div>
                      </td>
                      <td>
                        <div className="session-meta">{session.options.modelProfile}</div>
                        <div className="session-meta">
                          {session.options.latencyToleranceMs} ms
                        </div>
                      </td>
                      <td className="session-actions">
                        <button
                          type="button"
                          className="secondary-button"
                          onClick={() => {
                            setSelectedSessionId(session.id);
                            setStatusEvents([]);
                          }}
                          disabled={isSelected}
                        >
                          {isSelected ? "Monitoring" : "Monitor"}
                        </button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section>
        <h2>Live session status</h2>
        <p>
          Subscribe to WebSocket updates published by the worker and surfaced by
          <code> /sessions/{{id}}/events </code> to observe ingestion progress in
          real time.
        </p>
        {selectedSession ? (
          <div className="selection-banner">
            <span>
              Monitoring <strong>{selectedSession.id}</strong>
            </span>
            <button
              type="button"
              className="ghost-button"
              onClick={() => setSelectedSessionId(null)}
            >
              Stop monitoring
            </button>
          </div>
        ) : (
          <p className="empty-state">
            Select a session above to open a live status stream.
          </p>
        )}
        <div className="connection-status">
          <span className={`connection-pill ${connectionState}`}>
            {connectionLabel(connectionState)}
          </span>
          {connectionError ? (
            <span className="connection-error">{connectionError}</span>
          ) : null}
        </div>
        {statusEvents.length === 0 ? (
          <p className="empty-state">
            Status updates will appear here once the worker publishes events.
          </p>
        ) : (
          <ul className="status-events">
            {statusEvents.map((event, index) => (
              <li key={`${event.timestamp}-${event.stage}-${event.state}-${index}`}>
                <article className="status-event">
                  <header className="status-event-header">
                    <span className="status-event-stage">{event.stage}</span>
                    <span className="status-event-timestamp">
                      {formatTimestamp(event.timestamp)}
                    </span>
                  </header>
                  <div className="status-event-state">{event.state}</div>
                  {event.detail ? (
                    <p className="status-event-detail">{event.detail}</p>
                  ) : null}
                </article>
              </li>
            ))}
          </ul>
        )}
      </section>
    </main>
  );
}
