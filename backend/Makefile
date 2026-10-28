# Words after `cli` are the console command. Flags go after `--`:
#   make cli migrate up
#   make cli -- create-user --email admin@example.com --password password123
ifeq ($(firstword $(MAKECMDGOALS)),cli)
  CLI_ARGS ?= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  $(foreach arg,$(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)),$(eval $(arg):;@:))

.PHONY: cli
cli:
	go run ./cmd/cli $(CLI_ARGS)
else

run:
	go run ./cmd/api

dev:
	air

build:
	go build -o bin/api ./cmd/api

build-cron:
	go build -o bin/gogo-cron ./cmd/cron

build-cli:
	go build -o bin/gogo-cli ./cmd/cli

run-cron:
	go run ./cmd/cron

run-cron-test-db:
	go run ./cmd/cron --test-db

run-test-db:
	go run ./cmd/api --test-db

# Local Mailpit in Docker (see docker-compose.yml)
up:
	docker compose up -d --wait

down:
	docker compose down

docker-build:
	docker build -t gogo-api .

sqlc:
	sqlc generate

sqlc-install:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

swagger:
	swag init --parseDependency --parseInternal --requiredByDefault -g cmd/api/main.go

# Every test creates its own temporary SQLite database, so tests need no setup
test:
	go test ./tests/unit/... ./tests/integration/...

test-unit:
	go test ./tests/unit/...

test-integration:
	go test ./tests/integration/...

test-verbose:
	go test -v ./tests/unit/... ./tests/integration/...

test-coverage:
	go test -cover ./tests/unit/... ./tests/integration/...

# TEST_DATABASE_PATH is the database of --test-db, which the frontend E2E suite runs against
test-db-setup:
	go run ./cmd/cli --test migrate up

test-db-reset:
	go run ./cmd/cli --test migrate reset
	go run ./cmd/cli --test migrate up

test-migrate-up:
	go run ./cmd/cli --test migrate up

test-migrate-down:
	go run ./cmd/cli --test migrate down

test-migrate-status:
	go run ./cmd/cli --test migrate status

fmt:
	go fmt ./...

# Load .env file if it exists
ifneq (,$(wildcard .env))
    include .env
    export
endif

# Migration commands (DATABASE_PATH from .env or environment)
migrate-up:
	go run ./cmd/cli migrate up

migrate-down:
	go run ./cmd/cli migrate down

migrate-status:
	go run ./cmd/cli migrate status

migrate-reset:
	go run ./cmd/cli migrate reset

migrate-create:
	@read -p "Enter migration name: " name; \
	go run ./cmd/cli migrate create $$name

# CLI test command
cli-test:
	go run ./cmd/cli test

cli-test-db:
	go run ./cmd/cli --test test

# Usage: make cli-create-user EMAIL=admin@example.com PASSWORD=secret123 [NAME="Admin"] [ROLE=super-admin]
cli-create-user:
	go run ./cmd/cli create-user --email "$(EMAIL)" --password "$(PASSWORD)" $(if $(NAME),--name "$(NAME)") $(if $(ROLE),--role "$(ROLE)")

# Accounts and sample data for an empty development database
seed:
	go run ./cmd/cli seed

# Usage: make cli-mail TO=you@example.com
cli-mail:
	go run ./cmd/cli mail -to "$(TO)"

# CLI help
cli-help:
	go run ./cmd/cli help

tidy:
	go mod tidy

clean:
	rm -rf bin/ tmp/

air-install:
	go install github.com/air-verse/air@latest

# Supervisord commands
supervisor-restart:
	sudo supervisorctl restart gogo-api

supervisor-status:
	sudo supervisorctl status gogo-api

supervisor-stop:
	sudo supervisorctl stop gogo-api

supervisor-start:
	sudo supervisorctl start gogo-api

supervisor-logs:
	sudo tail -f /var/log/app/app.log

supervisor-error-logs:
	sudo tail -f /var/log/app/app-error.log

help:
	@echo "Gogo API Template - available targets:"
	@echo ""
	@echo "  Development:"
	@echo "    up                  Start Mailpit in Docker"
	@echo "    down                Stop them"
	@echo "    docker-build        Build the API image"
	@echo "    run                 Start API server"
	@echo "    run-test-db         Start API with TEST_DATABASE_PATH"
	@echo "    dev                 Hot reload (air)"
	@echo "    build               Build API binary"
	@echo "    build-cron          Build cron binary (only when ENABLE_SCHEDULER=false)"
	@echo "    build-cli           Build CLI binary"
	@echo "    fmt                 Format Go code"
	@echo "    tidy                go mod tidy"
	@echo ""
	@echo "  Database:"
	@echo "    migrate-up          Apply migrations"
	@echo "    migrate-down        Roll back one migration"
	@echo "    migrate-status      Show migration status"
	@echo "    migrate-reset       Reset all migrations"
	@echo "    migrate-create      Create a new migration"
	@echo "    sqlc                Generate sqlc code"
	@echo "    sqlc-install        Install sqlc CLI"
	@echo "    seed                Accounts and sample data for an empty development database"
	@echo "    cli                 Run a console command (make cli migrate up)"
	@echo "    cli-create-user     Create a user (EMAIL=, PASSWORD=, NAME=, ROLE=)"
	@echo "    cli-mail            Send a test email (TO=)"
	@echo ""
	@echo "  Testing:"
	@echo "    test                Run all tests (each test gets its own SQLite file)"
	@echo "    test-unit           Run unit tests"
	@echo "    test-integration    Run integration tests"
	@echo "    test-verbose        Run tests with -v"
	@echo "    test-coverage       Run tests with coverage"
	@echo "    test-db-setup       Apply migrations to TEST_DATABASE_PATH (E2E)"
	@echo "    test-db-reset       Reset and re-migrate TEST_DATABASE_PATH"
	@echo ""
	@echo "  Docs / Cron / Deploy:"
	@echo "    swagger             Generate Swagger docs"
	@echo "    run-cron            Run the scheduler as its own process"
	@echo "    supervisor-restart  Restart supervisord program"
	@echo "    help                Show this help"

.PHONY: up down docker-build seed run dev build build-cron build-cli run-cron run-cron-test-db run-test-db sqlc sqlc-install swagger test test-unit test-integration test-verbose test-coverage test-db-setup test-db-reset test-migrate-up test-migrate-down test-migrate-status fmt tidy clean air-install migrate-up migrate-down migrate-status migrate-reset migrate-create cli-test cli-test-db cli-create-user cli-mail cli-help supervisor-restart supervisor-status supervisor-stop supervisor-start supervisor-logs supervisor-error-logs help

endif
