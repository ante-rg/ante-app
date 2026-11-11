# ante-app

A monitoring system for the responsible-gambling team of a casino group. Licensed brands send player game events; the system scores each player's recent behaviour and returns the score with its top three reasons for a human to review. It exists to test whether a model finds loss-chasing players earlier than today's fixed rules. The score is a signal for review, not a diagnosis.

University group project (TSI/UWE MSc, AI Group Project UFCE7W-18-M). Components: a Go API, a Python scorer (FastAPI) and a React dashboard. The architecture is described in [ante-docs](https://github.com/ante-rg/ante-docs/blob/main/app/architecture.md).

## Prerequisites

- Go 1.27
- Node 24 and npm
- uv (Python 3.12 is managed by uv)
- Docker (needed for `make dev` and `make up`; database tests skip locally without Docker and run in CI)

## Common commands

```
make dev    # Postgres and scorer in Docker, API and web locally; web on the Vite port, proxying to :8080
make up     # full stack in Docker, built from source, on :8080
make test   # Go, Python and web tests
make lint   # golangci-lint, ruff, Biome and tsc
make gen    # regenerate code from openapi.yaml
```

`openapi.yaml` is the API contract: change it first, run `make gen`, then implement.

See [CLAUDE.md](CLAUDE.md) for the project rules and conventions.
