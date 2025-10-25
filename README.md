# Streamlation
Watch and listen in your language

## Getting Started

### Backend API

The backend HTTP server lives in `apps/api`. It exposes a simple health check and is designed to evolve into the full translation orchestration API described in the project plans.

```bash
cd apps/api
# Start the development server (defaults to :8080)
go run ./cmd/server
```

Set `APP_SERVER_ADDR` to override the listen address.

## Documentation

- [Final Architectural Plan](docs/final-architectural-plan.md)
- [Baseline Plan](docs/translation-streaming-plan.md)
- [Implementation Plan](docs/implementation-plan.md)
