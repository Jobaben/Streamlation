import Link from "next/link";

const milestones = [
  {
    title: "Backend health checks",
    description: "Go API exposes structured logging and readiness endpoints for Docker orchestration."
  },
  {
    title: "Worker heartbeats",
    description: "Background processor keeps Redis-ready loop alive while awaiting queue wiring."
  },
  {
    title: "Shared schemas",
    description: "JSON schemas document session contracts shared across services."
  },
  {
    title: "CI foundations",
    description: "Linting and unit tests execute through GitHub Actions with Go and Next.js tasks."
  }
];

export default function HomePage() {
  return (
    <main>
      <h1>Streamlation Platform</h1>
      <p>
        Phase one is complete: the monorepo now includes a Go API, worker, and Next.js
        frontend, each wired for local development, Docker-based orchestration, and
        shared schema contracts.
      </p>
      <p>
        Explore the <Link href="https://github.com/golang/go/wiki/Modules">Go modules guide</Link>
        {" "}or review the implementation plan in the docs directory to dive deeper
        into the roadmap.
      </p>
      <ul>
        {milestones.map((milestone) => (
          <li key={milestone.title}>
            <strong>{milestone.title}:</strong> {milestone.description}
          </li>
        ))}
      </ul>
    </main>
  );
}
