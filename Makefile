.DEFAULT_GOAL := help
SHELL         := /bin/sh

SERVER_DIR    := server
WEB_DIR       := web
HELM_DIR      := deploy/helm/confero
COMPOSE_FILE  := deploy/compose/docker-compose.yml

.PHONY: help
help: ## Display available targets
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} \
	     /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2 }' \
	     $(MAKEFILE_LIST)

# --------------------------------------------------------------------------- #
# Code generation                                                              #
# --------------------------------------------------------------------------- #

.PHONY: generate
generate: ## Regenerate OpenAPI stubs, sqlc queries, and TypeScript client
	@echo "==> Running code generators..."
	@$(MAKE) generate-api
	@$(MAKE) generate-sqlc
	@$(MAKE) generate-ts-client
	@echo "==> Done. Run 'git status' to see generated changes."

.PHONY: generate-api
generate-api: ## Generate Go server stubs from api/openapi.yaml
	@echo "  [api] oapi-codegen ..."
	cd $(SERVER_DIR) && go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen \
		-config oapi-codegen.yaml ../api/openapi.yaml

.PHONY: generate-sqlc
generate-sqlc: ## Generate repository code from SQL queries (requires sqlc)
	@if command -v sqlc >/dev/null 2>&1; then \
		echo "  [sqlc] sqlc generate ..."; \
		cd $(SERVER_DIR) && sqlc generate; \
	else \
		echo "  [sqlc] sqlc not installed — skipping (install: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest)"; \
	fi

.PHONY: generate-ts-client
generate-ts-client: ## Generate TypeScript client from api/openapi.yaml
	@echo "  [ts]  openapi-ts ..."
	cd $(WEB_DIR) && node_modules/.bin/openapi-ts

# --------------------------------------------------------------------------- #
# Linting                                                                      #
# --------------------------------------------------------------------------- #

.PHONY: lint
lint: go-lint web-lint openapi-lint helm-lint ## Run all linters

.PHONY: go-lint
go-lint: ## Lint Go code (requires golangci-lint)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "==> golangci-lint ..."; \
		cd $(SERVER_DIR) && golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; install from https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

.PHONY: web-lint
web-lint: ## Lint TypeScript/React code
	@echo "==> pnpm lint ..."
	cd $(WEB_DIR) && pnpm lint

.PHONY: openapi-lint
openapi-lint: ## Lint OpenAPI spec (requires @redocly/cli via npx)
	@echo "==> redocly lint ..."
	npx --yes @redocly/cli@latest lint api/openapi.yaml

.PHONY: helm-lint
helm-lint: ## Lint the Helm chart
	@echo "==> helm lint ..."
	helm lint $(HELM_DIR)

# --------------------------------------------------------------------------- #
# Testing                                                                      #
# --------------------------------------------------------------------------- #

.PHONY: test
test: go-test web-test helm-test ## Run all tests

.PHONY: go-test
go-test: ## Run Go tests with the race detector
	@echo "==> go test -race ./..."
	cd $(SERVER_DIR) && go test -race ./...

.PHONY: web-test
web-test: ## Run web tests (Vitest)
	@echo "==> pnpm test ..."
	cd $(WEB_DIR) && pnpm test

.PHONY: helm-test
helm-test: ## Run helm-unittest tests for the Helm chart
	@echo "==> helm unittest ..."
	helm unittest -f 'tests/unit/*_test.yaml' $(HELM_DIR)

.PHONY: web-typecheck
web-typecheck: ## Type-check TypeScript without emitting
	@echo "==> tsc --noEmit ..."
	cd $(WEB_DIR) && pnpm typecheck

# --------------------------------------------------------------------------- #
# Building                                                                     #
# --------------------------------------------------------------------------- #

.PHONY: build
build: build-server build-web ## Build Docker images for server and web

.PHONY: build-server
build-server: ## Build the confero-server Docker image
	@echo "==> Building server image ..."
	docker build \
		--file $(SERVER_DIR)/Dockerfile \
		--tag confero-server:dev \
		--build-arg VERSION=dev \
		--build-arg COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown) \
		.

.PHONY: build-web
build-web: ## Build the confero-web Docker image
	@echo "==> Building web image ..."
	docker build \
		--file $(WEB_DIR)/Dockerfile \
		--tag confero-web:dev \
		.

# --------------------------------------------------------------------------- #
# Local development                                                            #
# --------------------------------------------------------------------------- #

.PHONY: dev
dev: ## Start all services via docker-compose (Postgres, Keycloak, MailHog, server, web)
	docker compose -f $(COMPOSE_FILE) up --build

.PHONY: dev-services
dev-services: ## Start only backing services (Postgres, Keycloak, MailHog)
	docker compose -f $(COMPOSE_FILE) up postgres keycloak mailhog

.PHONY: dev-down
dev-down: ## Stop and remove compose containers
	docker compose -f $(COMPOSE_FILE) down

# --------------------------------------------------------------------------- #
# Database migrations                                                          #
# --------------------------------------------------------------------------- #

MIGRATE := migrate -path $(SERVER_DIR)/db/migrations -database "$(CONFERO_DATABASE_URL)"

.PHONY: migrate-up
migrate-up: ## Apply all pending migrations (requires CONFERO_DATABASE_URL)
	$(MIGRATE) up

.PHONY: migrate-down
migrate-down: ## Roll back one migration (requires CONFERO_DATABASE_URL)
	$(MIGRATE) down 1

.PHONY: migrate-new
migrate-new: ## Create a new migration pair: make migrate-new name=<description>
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-new name=<description>"; exit 1; fi
	migrate create -ext sql -dir $(SERVER_DIR)/db/migrations -seq $(name)

# --------------------------------------------------------------------------- #
# Utilities                                                                    #
# --------------------------------------------------------------------------- #

.PHONY: install-tools
install-tools: ## Install required Go tools
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin latest

.PHONY: web-install
web-install: ## Install web dependencies
	cd $(WEB_DIR) && pnpm install
