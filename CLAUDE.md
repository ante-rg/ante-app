# ante-app

Monitoring system for the responsible-gambling (RG) team of a casino group: Go API, Python scorer, React dashboard. University group project (TSI/UWE MSc, AI Group Project UFCE7W-18-M, autumn 2026). Public repository in the `ante-rg` GitHub organisation.

## What we are building and why

Topic: early detection of addictive gambling behaviour. The framing: a casino group runs five licensed brands (P1–P5). Its regulator's licence conditions require it to monitor its players, so the group's RG team gets a system that scores each player's recent behaviour and returns the score with reasons. The RG team reviews the cases; every user sees all five brands.

The research question: does a model ranking find players with loss-chasing behaviour **earlier** than the fixed rules operators use today (for example "deposits over 2,000 € in 30 days")? The dashboard exists to show and test exactly that.

The data: an anonymised export of 100 player accounts from 5 brands (P1–P5, 20 each), about 7.6 million events (bets, results, balances, deposits, withdrawal requests, bonuses). No calendar dates, only time since each player's first event. No ground-truth labels: the label is a proxy defined by the data science team (share of "loss-chasing" sessions: 2 h or longer, 3 or more deposits, net loss). The score is a signal for human review, never a diagnosis; the UI must not word it as one.

Objectives and milestones come from `../ante-docs/project_plan.md`, which wins on every date and on the content of every objective:

| Objective | Date | Milestone | What it says |
|---|---|---|---|
| O1 | 8 Nov 2026 | MS2 | One-command pipeline from raw events to sessions and behavioural markers; label and evaluation protocol frozen; rule baseline scored. |
| O2 | 22 Nov 2026 | MS3 | At least two ML models compared with the rule baseline at the same review capacity (early recall, median lead time). |
| O3 | 6 Dec 2026 | MS4 | Deployed API and dashboard that score a replayed event stream, return each score with its top three reasons, and pass tests that API scores match the offline model. **This repository.** |
| O4 | 15 Dec 2026 | MS6 | Documented pipeline and model card (data protection, ethical risks, limitations), verified by an independent rerun. |

Other milestones: MS1 data and label source confirmed (30 Oct), MS5 results frozen (10 Dec 2026). What the app must show:

- MS3, 22 Nov: walking-skeleton MVP serves baseline scores end to end, with CI/CD and one-command deployment (D5.1, D5.2 first version, D5.3).
- MS4, 6 Dec: the selected model's scores with top three reasons over a replayed stream; parity tests pass; blind review mode, where the API omits the fields that would reveal which list a case came from.
- MS6, 15 Dec: live demo at the final presentation, run on this app.

## Team and ownership

- Volodymyr Stishkovskyi: Software Developer, ML Engineer. Owns this repository: Go API, React dashboard, deployment, CI/CD (WP5).
- Jurijs Kaminskis: Product Owner, Business Intelligence. Prioritises the backlog, supplies domain knowledge, owns the data and casino side.
- Andrejs Fjodorovs: Lead Data Scientist (modelling, evaluation).
- Daniils Buts: Process Expert, Data Scientist (Data Driven Scrum events, synthesis).
- Arturs Greķe: Data Engineer (data contract, quality checks, pipeline).

The data science work lives in a separate repository, `ante-research`. It will publish the feature/model Python package and the `artifacts/<model_version>/` folder this app loads. Neither exists yet; the package name is left open until `ante-research` names it. The process is Data Driven Scrum; backlog cards are in the GitHub Project of `ante-rg`. Roles are as in `../ante-docs/project_charter.md`.

## Current state

The repository is a runnable skeleton: Go API with `/healthz` and an empty embedded migrations folder, Python scorer with `/healthz`, React app with a Health screen, Docker Compose files, and a CI workflow that also publishes images. Every operation in `openapi.yaml` except `getHealth` answers `501 Not Implemented` until its feature task.

The data artifacts from `ante-research` do not exist yet. Until the data science team publishes them, use dummy artifacts: a Go command (`api/cmd/fakeartifacts`, later) writes a deterministic set in the format of `../ante-docs/app/architecture.md` section 4 to `artifacts/dummy-v0/` (git-ignored). Keep the loader strict about that format so real artifacts replace the dummy ones without code changes.

Suggested order: Postgres schema and migrations → fake artifacts and loader → read endpoints with `as_of_day` → web: queue and player case → reviews and blind mode → ingest and scorer → Cloudflare hostnames and the `deploy` job → operators and about screens.

## Read before changing anything

Design docs live in the [`ante-docs`](https://github.com/ante-rg/ante-docs) repository. Locally it is cloned next to this one, at `../ante-docs`. Where `ante-docs` and this file disagree on the product framing (casino-group RG team, five brands), the framing here is the agreed one.

- `../ante-docs/app/architecture.md`: components, replay semantics, data handoff, database, blind review, auth.
- `openapi.yaml`: the API contract. It is the source of truth; code is generated from it.
- `../ante-docs/app/frontend-decisions.md`: agreed UI decisions. Do not reverse one without asking.
- `../ante-docs/app/design/ante-dashboard-mockup.html`: the high-fidelity mockup of the queue and the player case. Match its layout, copy and components. Its data is invented.
- `web/src/styles/tokens.css`: the only place colours, fonts and spacing are defined.
- `../ante-docs/project_plan.md` and `../ante-docs/project_charter.md`: objectives, milestones, roles.

## Layout

Entries marked "(later)" do not exist yet.

```
api/            Go module (github.com/ante-rg/ante-app/api)
  cmd/api/        HTTP server, also serves web/dist
  cmd/loader/     (later) loads artifacts/<model_version>/ into Postgres, reads parquet
  cmd/fakeartifacts/ (later) writes dummy artifacts for development
  internal/config/ environment config
  internal/db/    pgx pool, goose migrations applied on start
  internal/gen/   oapi-codegen output. Never edit by hand.
  internal/http/  chi router, handlers, middleware (request id, slog)
  internal/testdb/ testcontainers Postgres for tests
  internal/store/ (later) sqlc queries and generated code
  internal/scorer/ (later) client for the Python scorer
  migrations/     goose SQL migrations, embedded in the binary
scorer/         Python 3.12 FastAPI service, managed with uv
  src/scorer/     app code
  tests/
web/            React + TypeScript (Vite)
  src/api/        openapi-typescript types and a thin fetch client
  src/screens/    Health now; Queue, PlayerCase, Operators, About later
  src/styles/     tokens.css, base.css
  src/test/       Vitest setup
deploy/         docker-compose.yml, local.yml, .env.example
.github/workflows/ ci.yml (lint, tests, image smoke test; `publish` job pushes images on main after all checks pass)
fixtures/       (later) synthetic players for tests and the contract test
```

## Stack and libraries

- Go 1.27: chi, pgx/v5, sqlc, goose, oapi-codegen (strict server), log/slog. No ORM. `oapi-codegen`, `goose` and `sqlc` are `tool` directives in `api/go.mod`, run with `go tool`.
- Python 3.12: FastAPI, uvicorn, uv. The data science package is added, pinned by git tag, once `ante-research` publishes it.
- Web: React 19, TypeScript strict, Vite, React Router, TanStack Query, Vitest, Testing Library, Biome (lint and format), openapi-typescript, npm. Radix Primitives and visx arrive with the first feature that uses them. Plain CSS with custom properties. No Tailwind, no component kits, no CSS-in-JS.
- PostgreSQL 16. Node 24 LTS.

Ask before adding a dependency that is not on this list. goose and testcontainers-go are already direct dependencies of `api`. The one approved addition still pending is parquet-go, for the loader.

## Commands

```
make dev          # postgres + scorer in docker, api (go run) and web (Vite) locally
make up           # full stack in docker, built from source, on :8080
make gen          # oapi-codegen and openapi-typescript
make test         # go test ./..., pytest, vitest
make lint         # golangci-lint v2.15.0 (go run), ruff check + format check, Biome + tsc --noEmit
make build        # build the compose images from source
```

`make contract` (contract test: ingest fixtures, compare scores with expected) and `make load MODEL=<version>` (run the loader against `artifacts/<version>`) are added by later tasks. `make gen` will also run sqlc once its config exists.

Run `make gen` after any change to `openapi.yaml` (and, once added, `api/internal/store/queries/*.sql`), and commit the generated code.

## Deployment

The app runs on the team's own server with Docker Compose: `postgres`, `api` (serves the web build, published on `127.0.0.1:8080` only), `scorer` (internal network only) and `watchtower` (`nickfedor/watchtower`, since `containrrr/watchtower` is archived). Public access goes through the Cloudflare Tunnel already running on the host, protected by Cloudflare Access. There is no `cloudflared` container in compose.

On `main`, the `publish` job in `ci.yml` builds and pushes, only after the other CI jobs pass, `ghcr.io/ante-rg/ante-app` and `ghcr.io/ante-rg/ante-app-scorer` for `linux/amd64`. A GitHub Actions `deploy` job (added with the Cloudflare setup) then calls Watchtower's HTTP API and polls `/healthz` until `version` equals the commit SHA. Cloudflare hostnames, Access apps and the service token are set up outside this repository.

## Rules

- Contract first: change `openapi.yaml`, run `make gen`, then implement. Handlers implement the generated interface.
- "Day" is days since the player's own first activity. There are no calendar dates anywhere. Never convert days to dates.
- Every dashboard GET takes `as_of_day`. Data after that day must never reach the client.
- Blind mode is enforced in the API by omitting fields, never by hiding them in the UI.
- `reviews` is append-only. No update or delete endpoints.
- The backend never writes reason text. It returns what the DS artifacts contain.
- Never commit `*.parquet`, `artifacts/`, `.env`, or any real data. Fixtures are synthetic.
- Never log event payloads or review notes.
- Money is in euros, shown with the € sign after the number with a space: `640 €`.

## Frontend conventions

- UI copy is English, sentence case, plain verbs. Buttons say what happens: "Save review", not "Submit".
- Colours, type and spacing come only from tokens in `tokens.css`. No literal colours in components.
- Risk band is shown by the `BandChip` component only: filled for high, outlined for elevated, plain text for low. Never by colour alone.
- Numbers in tables use `font-variant-numeric: tabular-nums`.
- Replay day lives in the URL (`?day=120`) and in one React context. All queries include it in the query key.
- Desktop-first from 1280 px. Must stay usable at 400 px without horizontal page scroll.
- Keyboard: visible focus on everything; in the queue, `j`/`k` move between rows, `Enter` opens the case.

## Testing

- Go: table tests for handlers against a real Postgres (testcontainers), not mocks. These tests run in CI, where Docker is available. Local Docker is optional: some team machines cannot run it, and the tests then rely on CI.
- Every endpoint has a test for `as_of_day` cut-off and, where relevant, for blind mode omitting fields.
- Web: Vitest + Testing Library for components with logic; no snapshot tests.
- Scorer: pytest.

## Commits and PRs

- Conventional commits (`feat(api): ...`, `fix(web): ...`).
- One concern per PR. CI must be green. PR description says which backlog card it closes.
