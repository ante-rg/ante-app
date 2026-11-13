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
make gen    # regenerate code from openapi.yaml and the sqlc queries
```

`openapi.yaml` is the API contract: change it first, run `make gen`, then implement.

See [CLAUDE.md](CLAUDE.md) for the project rules and conventions.

## Server

The app runs on the team server with Docker Compose, behind the Cloudflare Tunnel that already runs on the host.

| What | Where |
| --- | --- |
| Dashboard | `https://ante.stishkovskyi.com` (Cloudflare Access, team emails) → `127.0.0.1:8180` |
| Watchtower HTTP API | `https://ante-deploy.stishkovskyi.com` (Cloudflare Access, service token only) → `127.0.0.1:8181` |
| Compose files | `~/ante/docker-compose.yml` and `~/ante/.env` (mode 600) on the server |
| Tunnel config | `/etc/cloudflared/config.yml`, tunnel `myserver`, systemd unit `cloudflared` |

Every green push to `main` runs `publish` and then `deploy` in `ci.yml`. `deploy` calls Watchtower's `POST /v1/update`, then polls `/healthz` until `version` equals the commit SHA. It needs:

- secrets `CF_ACCESS_CLIENT_ID`, `CF_ACCESS_CLIENT_SECRET` (the Access service token) and `WATCHTOWER_TOKEN` (the value in `~/ante/.env`);
- variables `APP_URL` and `DEPLOY_URL`.

Runbook:

```
cd ~/ante
docker compose ps                 # state of postgres, api, scorer, watchtower
docker compose logs -f api        # API logs
docker compose pull && docker compose up -d   # manual deploy of :latest
```

`deploy` only swaps images. A change to `deploy/docker-compose.yml` has to be copied to `~/ante/` by hand, followed by `docker compose up -d`. `.env` is created once on the server with `openssl rand -hex 24` for the database password and is never committed.
