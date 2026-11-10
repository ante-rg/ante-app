# ante-app

Monitoring system for a gambling regulator: Go API, Python scorer, React dashboard. University group project (TSI/UWE MSc, AI Group Project UFCE7W-18-M, autumn 2026). Public repository in the `ante-rg` GitHub organisation.

## What we are building and why

Topic: early detection of addictive gambling behaviour. The framing: a national gambling regulator commissions a system; licensed online casinos must send player game events to it as a licence condition, and the system returns a risk score with reasons that a human supervisory officer reviews.

The research question: does a model ranking find players with loss-chasing behaviour **earlier** than the fixed rules operators use today (for example "deposits over 2,000 € in 30 days")? The dashboard exists to show and test exactly that.

The data: an anonymised export of 100 player accounts from 5 operators (P1–P5, 20 each), about 7.6 million events (bets, results, balances, deposits, withdrawal requests, bonuses). No calendar dates, only time since each player's first event. No ground-truth labels: the label is a proxy defined by the data science team (share of "loss-chasing" sessions: 2 h or longer, 3 or more deposits, net loss). The score is a signal for human review, never a diagnosis; the UI must not word it as one.

Proposal objectives, with deadlines:
- O1, 6 Nov 2026: data pipeline, proxy label, fixed-rule baseline (data science team).
- O2, 27 Nov 2026: risk model, precision/recall in top 20% for 7/14/30-day windows, leave-one-operator-out validation (data science team).
- O3, 4 Dec 2026: deployed API and dashboard that score a replayed event stream with top-3 reasons, contract tests passing. **This repository.**
- O4, 10 Dec 2026: blind team review of 40 cases (model list vs rule list) and demo. Uses the blind mode of this dashboard.
- Final portfolio: 15 Dec 2026. The viva demo runs on this app.

## Team and ownership

- Volodymyr Stishkovskyi: developer and DevOps. Owns this repository: Go API, React dashboard, deployment, CI/CD.
- Andrejs Fjodorovs: Process Expert and lead data scientist (model, early-detection experiments).
- Daniils Buts: Product Owner (regulation, ethics, label, fixed-rule baseline).
- Arturs Greķe: data engineer (pipeline, data quality, feature tables, synthetic dataset).
- Jurijs Kaminskis: data scientist (behavioural markers, SHAP explanations, evaluation).

The data science work lives in a separate repository, `ante-research`, which also publishes the `rg_features` Python package and the `artifacts/<model_version>/` folder this app loads. The process is Data Driven Scrum; backlog cards are in the GitHub Project of `ante-rg`.

## Current state

The repository starts empty apart from these specs. Nothing is implemented yet. The data artifacts from `ante-research` do not exist yet either, so build against synthetic fixtures in `fixtures/` that follow the format in `ante-docs/app/architecture.md` section 4, and keep the loader strict about that format.

Suggested order: repo skeleton and `make` targets → Postgres migrations → loader with synthetic fixtures → read endpoints with `as_of_day` → web: queue and player case → reviews and blind mode → ingest + scorer stub → Docker Compose, CI, deploy → operators and about screens.

## Read before changing anything

Design docs live in the [`ante-docs`](https://github.com/ante-rg/ante-docs) repository. Locally it is cloned next to this one, at `../ante-docs`.

- `../ante-docs/app/architecture.md`: components, replay semantics, data handoff, database, blind review, auth.
- `openapi.yaml`: the API contract. It is the source of truth; code is generated from it.
- `../ante-docs/app/frontend-decisions.md`: agreed UI decisions. Do not reverse one without asking.
- `../ante-docs/app/design/ante-dashboard-mockup.html`: the high-fidelity mockup of the queue and the player case. Match its layout, copy and components. Its data is invented.
- `web/src/styles/tokens.css`: the only place colours, fonts and spacing are defined.
- `../ante-docs/proposal.md`: the submitted project proposal, for background.

## Layout (target)

```
api/            Go module (github.com/ante-rg/ante-app/api)
  cmd/api/        HTTP server, also serves web/dist
  cmd/loader/     loads artifacts/<model_version>/ into Postgres
  internal/http/  chi router, handlers, middleware (auth, audit, request id)
  internal/gen/   oapi-codegen output. Never edit by hand.
  internal/store/ sqlc queries and generated code
  internal/scorer/ client for the Python scorer
  migrations/     goose SQL migrations
scorer/         Python FastAPI service; depends on rg_features from ante-research, pinned by git tag
web/            React + TypeScript (Vite)
  src/api/        openapi-typescript types and a thin fetch client
  src/screens/    Queue, PlayerCase, Operators, About
  src/components/ shared components on Radix Primitives
  src/styles/     tokens.css, base.css
deploy/         docker-compose.yml, local.yml, cloudflared config
fixtures/       synthetic players for tests and the contract test
```

## Stack and libraries

- Go 1.23: chi, pgx/v5, sqlc, goose, oapi-codegen (strict server), log/slog. No ORM.
- Python 3.12: FastAPI, uvicorn, psycopg 3, rg_features, lightgbm, shap. Managed with uv.
- Web: React 18, TypeScript strict, Vite, React Router, TanStack Query, Radix Primitives, visx. Plain CSS with custom properties. No Tailwind, no component kits, no CSS-in-JS.
- PostgreSQL 16.

Ask before adding a dependency that is not on this list.

## Commands

```
make dev          # postgres + scorer in docker, api and web locally with hot reload
make gen          # oapi-codegen, sqlc, openapi-typescript
make test         # go test ./..., pytest, vitest
make lint         # golangci-lint, ruff, eslint + tsc --noEmit
make contract     # contract test: ingest fixtures, compare scores with expected
make load MODEL=v0.3.0   # run the loader against artifacts/v0.3.0
```

Run `make gen` after any change to `openapi.yaml` or `api/internal/store/queries/*.sql`, and commit the generated code.

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

- Go: table tests for handlers against a real Postgres (testcontainers), not mocks.
- Every endpoint has a test for `as_of_day` cut-off and, where relevant, for blind mode omitting fields.
- Web: vitest + Testing Library for components with logic; no snapshot tests.

## Commits and PRs

- Conventional commits (`feat(api): ...`, `fix(web): ...`).
- One concern per PR. CI must be green. PR description says which backlog card it closes.
