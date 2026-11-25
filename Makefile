# base compose file requires the password even when local.yml overrides it
export POSTGRES_PASSWORD ?= ante
export POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://postgres:$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/postgres?sslmode=disable
COMPOSE := docker compose -f deploy/docker-compose.yml -f deploy/local.yml
LINT_GO := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

.PHONY: dev up gen lint test build artifacts load apikey baseline

gen:
	cd api && go tool oapi-codegen -config oapi-codegen.yaml ../openapi.yaml
	cd api && go tool sqlc generate
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
	(cd api && DATABASE_URL='postgres://postgres:$(POSTGRES_PASSWORD)@localhost:$(POSTGRES_PORT)/postgres?sslmode=disable' SCORER_URL=http://localhost:8000 AUTH_DEV_USER=dev@localhost go run ./cmd/api) & \
	(cd web && npm run dev) & \
	wait

artifacts:
	cd api && go run ./cmd/fakeartifacts

baseline:
	@test -n "$(IN)" || { echo "usage: make baseline IN=<export folder>"; exit 2; }
	cd api && go run ./cmd/baseline -in '$(abspath $(IN))'

load:
	@test -n "$(MODEL)" || { echo "usage: make load MODEL=<version> [DATABASE_URL=...]"; exit 2; }
	@cd api && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/loader -dir ../artifacts/$(MODEL)

apikey:
	@test -n "$(OPERATOR)" || { echo "usage: make apikey OPERATOR=P3 [DATABASE_URL=...]"; exit 2; }
	@cd api && DATABASE_URL='$(DATABASE_URL)' go run ./cmd/apikey -operator $(OPERATOR)
