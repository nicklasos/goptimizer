# Words after `cli` are the console command, not Make targets. Flags start
# with a dash, so they go after `--`:
#   make cli migrate up
#   make cli -- create-user --email admin@example.com --password password123
ifeq ($(firstword $(MAKECMDGOALS)),cli)
  CLI_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))
  $(foreach arg,$(CLI_ARGS),$(eval $(arg):;@:))

.PHONY: cli
cli: ## Run a console command (make cli migrate up)
	$(MAKE) -C backend cli CLI_ARGS='$(CLI_ARGS)'
else

.PHONY: help up down seed test test-backend test-frontend deploy-setup provision install-remote deploy-remote deploy

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'

up: ## Start Mailpit in Docker for local development
	$(MAKE) -C backend up

down: ## Stop them
	$(MAKE) -C backend down

seed: ## Fill an empty development database with sample accounts
	$(MAKE) -C backend seed

test: test-backend test-frontend ## Run every test

test-backend: ## Go unit and integration tests
	$(MAKE) -C backend test

test-frontend: ## Typecheck, lint, unit tests and Playwright
	$(MAKE) -C frontend test

deploy-setup: ## Install the Ansible collections the playbooks need
	cd deploy && ansible-galaxy collection install -r requirements.yml

provision: ## Set up a new server and deploy to it
	cd deploy && ansible-playbook provision.yml

install-remote: ## Add the application to a server that already has Go, nginx, supervisor (deploy/README.md)
	cd deploy && ansible-playbook install.yml

deploy-remote: ## Release the current code to the server with Ansible, from your machine
	cd deploy && ansible-playbook deploy.yml

# For a server set up with install.yml or by hand (deploy/MANUAL_INSTALL.md). The program name is the one
# in /etc/supervisor/conf.d: make deploy SUPERVISOR_PROGRAM=other-api. Empty skips the restart.
SUPERVISOR_PROGRAM ?= goptimizer-api

deploy: ## Run on the server: pull, build, migrate, restart
	git pull --ff-only
	cd backend && go build -o bin/api ./cmd/api && go build -o bin/goptimizer-cli ./cmd/cli
	cd backend && ./bin/goptimizer-cli migrate up
	cd frontend && npm ci --no-audit --no-fund && npm run build
	@if [ -n "$(SUPERVISOR_PROGRAM)" ]; then sudo supervisorctl restart $(SUPERVISOR_PROGRAM); fi

.DEFAULT_GOAL := help

endif
