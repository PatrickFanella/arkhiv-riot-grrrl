SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

.PHONY: help verify deps fmt-check typecheck lint test build compose-config open-pilot-check

help:
	@awk 'BEGIN {FS = ":.*##"; printf "Commands:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps:
	pnpm --dir web install --frozen-lockfile

verify: deps fmt-check typecheck lint test build compose-config open-pilot-check ## Run all checks

fmt:
	@find backend -name '*.go' -not -path '*/vendor/*' -print0 | xargs -0 -r gofmt -w

fmt-check:
	@test -z "$$(find backend -name '*.go' -not -path '*/vendor/*' -print0 | xargs -0 -r gofmt -l)" || (echo 'Run make fmt'; exit 1)

typecheck:
	pnpm --dir web run typecheck

lint:
	cd backend && go vet ./...
	pnpm --dir web run lint

test: test-operations
	cd backend && go test ./...
	pnpm --dir web run test

build:
	cd backend && go build -o bin/api ./cmd/app && go build -o bin/arkhiv ./cmd/arkhiv
	pnpm --dir web run build

compose-config:
	docker compose config --quiet

open-pilot-check:
	@test -f .gitea/ISSUE_TEMPLATE/open-pilot-task.yaml
	@grep -q 'Test Command' .gitea/ISSUE_TEMPLATE/open-pilot-task.yaml

.PHONY: fmt-check typecheck dev-api dev-web
dev-api: ## Start the Go service
	cd backend && go run ./cmd/app

dev-web: ## Start Vite with the API proxy
	pnpm --dir web dev

.PHONY: test-integration test-recovery test-browser
test-integration: ## Catalog, search, asset and recovery checks against disposable PostgreSQL
	@test -n "$$TEST_DATABASE_URL" || (echo 'Run with t3-dev --profile arkhiv test --command "make test-integration"'; exit 1)
	cd backend && go test -race ./internal/app -run TestArchiveIntegration -count=1

test-recovery: test-integration ## Restore and integrity checks in the integration suite

test-browser: ## Synthetic desktop and narrow-screen browser suite on an isolated network
	bash scripts/test-browser.sh

.PHONY: test-operations
test-operations: ## Isolated job, manifest, retention and failure checks
	python3 -m unittest discover -s scripts/operations -v

.PHONY: test-updates
test-updates: ## Complete source inventory and durable refresh integration checks
	@test -n "$$TEST_DATABASE_URL" || (echo 'Run with t3-dev --profile arkhiv test --command "make test-updates"'; exit 1)
	cd backend && go test -race ./internal/updates -count=1

.PHONY: test-restore-browser
test-restore-browser: ## Scheduled reader probe against an isolated synthetic stack
	bash scripts/test-reader-probe.sh

.PHONY: verify-core-pin
verify: verify-core-pin
verify-core-pin: ## Verify the independent Arkhiv engine receipt and deployment recipe
	python3 scripts/check_core_pin.py
	COLLECTION=isolated-proof ARCHIVE_ROOT=/tmp/arkhiv-proof DERIVATIVE_ROOT=/tmp/arkhiv-derivatives IMPORT_ROOT=/tmp/arkhiv-import SECRET_ROOT=/tmp/arkhiv-secrets docker compose --env-file deploy/archive-core.env -f deploy/core-compose.yml config --quiet
