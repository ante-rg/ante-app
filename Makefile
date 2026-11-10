# base compose file requires the password even when local.yml overrides it
export POSTGRES_PASSWORD ?= ante
COMPOSE := docker compose -f deploy/docker-compose.yml -f deploy/local.yml
LINT_GO := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

.PHONY: dev up gen lint test build

gen:
	cd api && go tool oapi-codegen -config oapi-codegen.yaml ../openapi.yaml
	cd web && npm run gen

lint:
	cd api && $(LINT_GO) run ./...
	cd scorer && uv run ruff check . && uv run ruff format --check .
	cd web && npm run lint

test:
	cd api && go test ./...
	cd scorer && uv run pytest
	cd web && npm test

build:
	$(COMPOSE) build

up:
	$(COMPOSE) up --build

dev:
	$(COMPOSE) up -d --wait postgres scorer
	trap 'kill 0' INT TERM EXIT; \
	(cd api && DATABASE_URL='postgres://postgres:$(POSTGRES_PASSWORD)@localhost:5432/postgres?sslmode=disable' SCORER_URL=http://localhost:8000 go run ./cmd/api) & \
	(cd web && npm run dev) & \
	wait
