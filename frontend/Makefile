.PHONY: help install dev build preview api-types api-types-check typecheck lint lint-fix test test-unit test-e2e test-e2e-ui test-e2e-headed clean playwright-browsers env-check db-test-setup

help: ## Show available commands
	@echo "Goptimizer frontend - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

install: ## Install dependencies
	npm install

dev: ## Start Vite dev server
	npm run dev

build: ## Typecheck and production build
	npm run build

preview: ## Preview production build
	npm run preview

api-types: ## Regenerate src/api/schema.ts from ../backend/docs/swagger.json (run `make swagger` in gogo first)
	npm run api:types

api-types-check: api-types ## Fail when src/api/schema.ts is out of date with the gogo swagger file
	git diff --exit-code -- src/api/schema.ts

typecheck: ## TypeScript check (src, configs and E2E)
	npm run typecheck

lint: typecheck ## Typecheck, then ESLint
	npm run lint

lint-fix: ## ESLint with --fix
	npm run lint -- --fix

test: lint test-unit test-e2e ## Everything: typecheck, lint, unit, E2E

test-unit: ## Run unit tests (node:test)
	npm run test:unit

test-e2e: ## Playwright E2E (starts gogo with --test-db and Vite on the E2E ports)
	npx playwright test

test-e2e-ui: ## Playwright UI mode
	npx playwright test --ui

test-e2e-headed: ## Playwright headed
	npx playwright test --headed

playwright-browsers: ## Install Playwright browsers
	npx playwright install

db-test-setup: ## Apply gogo migrations to TEST_DATABASE_PATH
	cd ../backend && $(MAKE) test-db-setup

env-check: ## Check local environment
	@echo "Node: $$(node --version)"
	@echo "NPM: $$(npm --version)"
	@if [ -f ".env" ]; then echo ".env exists"; else echo ".env missing (cp .env.example .env)"; fi

clean: ## Remove build/test artifacts
	rm -rf dist/ node_modules/.vite/ test-results/ playwright-report/

.DEFAULT_GOAL := help
