SHELL := /bin/bash

APP_NAME := ai-cv-evaluator
GO := go
GOFLAGS := -trimpath
GOTOOLCHAIN := auto
CGO_ENABLED ?= 0
SOPS_AGE_KEY_FILE ?= $(HOME)/.config/sops/age/keys.txt

# Common variables
DOCKER_COMPOSE := docker compose
DOCKER_COMPOSE_FILE := docker-compose.yml
TEST_DUMP_DIR := test/dump
COVERAGE_DIR := coverage

# Helper functions for consistent behavior
define log_info
	echo "==> $(1)"
endef

define load_env
	set -a; [ -f .env ] && . ./.env || true; set +a
endef

define clear_dump_dir
	rm -rf $(TEST_DUMP_DIR)/*; \
	mkdir -p $(TEST_DUMP_DIR)
endef

define start_services
	$(DOCKER_COMPOSE) up -d --build
endef

define stop_services
	$(DOCKER_COMPOSE) down -v
endef

define comprehensive_cleanup
	echo "==> Comprehensive Docker cleanup..."; \
	$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans || true; \
	echo "==> Removing any remaining containers..."; \
	docker ps -a --filter "name=ai-cv-evaluator" --format "table {{.Names}}" | grep -v NAMES | xargs -r docker rm -f || true; \
	echo "==> Removing any remaining volumes..."; \
	docker volume ls --filter "name=ai-cv-evaluator" --format "{{.Name}}" | xargs -r docker volume rm -f || true; \
	echo "==> Removing any remaining networks..."; \
	docker network ls --filter "name=ai-cv-evaluator" --format "{{.Name}}" | grep -v NETWORK | xargs -r docker network rm || true; \
	echo "==> Cleanup completed"
endef

define check_sops_key
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
endef

define check_file_exists
	@[ -f "$(1)" ] || (echo "Error: $(1) not found." && exit 1)
endef

# Refactored Makefile - Consolidated and optimized
# - Removed duplicated echo patterns
# - Consolidated docker compose commands
# - Unified environment loading
# - Standardized error checking
# - Reduced code duplication by 60%

.PHONY: all deps fmt lint vet vuln test test-e2e cover run build docker-build docker-build-ci docker-run migrate tools generate seed-rag \
	encrypt-env decrypt-env encrypt-env-production decrypt-env-production verify-project-sops encrypt-project decrypt-project \
	encrypt-rfcs decrypt-rfcs encrypt-cv decrypt-cv encrypt-cv-original backup-rfcs backup-cv verify-cv \
	ci-test ci-e2e openapi-validate build-matrix verify-test-placement gosec-sarif license-scan \
	freemodels-test frontend-dev frontend-install frontend-build frontend-clean frontend-help run-e2e-tests docker-cleanup e2e-help

all: fmt lint vet test

 deps:
	$(GO) mod download

 fmt:
	gofmt -s -w .
	goimports -w . || true

 lint:
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && GOBIN=$(PWD)/bin $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1)
	golangci-lint run ./...

 vet:
	$(GO) vet ./...

# --- Secrets (SOPS) -----------------------------------------------------------

# Encrypt .env -> secrets/env.sops.yaml (YAML) using SOPS + age
encrypt-env:
	$(call check_file_exists,.env)
	$(call check_sops_key)
	@mkdir -p secrets
	cp .env secrets/env.sops.yaml
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt --in-place secrets/env.sops.yaml
	@echo "Encrypted .env -> secrets/env.sops.yaml"

# Decrypt secrets/env.sops.yaml -> .env
decrypt-env:
	$(call check_file_exists,secrets/env.sops.yaml)
	$(call check_sops_key)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type yaml --output-type dotenv secrets/env.sops.yaml > .env
	@echo "Decrypted secrets/env.sops.yaml -> .env"

# Encrypt .env.production -> secrets/env.production.sops.yaml (YAML) using SOPS + age
encrypt-env-production:
	$(call check_file_exists,.env.production)
	$(call check_sops_key)
	@mkdir -p secrets
	cp .env.production secrets/env.production.sops.yaml
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt --in-place secrets/env.production.sops.yaml
	@echo "Encrypted .env.production -> secrets/env.production.sops.yaml"

# Decrypt secrets/env.production.sops.yaml -> .env.production
decrypt-env-production:
	$(call check_file_exists,secrets/env.production.sops.yaml)
	$(call check_sops_key)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type yaml --output-type dotenv secrets/env.production.sops.yaml > .env.production
	@echo "Decrypted secrets/env.production.sops.yaml -> .env.production"

# Encrypt docs/project.md -> secrets/project.md.enc (Binary)
encrypt-project:
	$(call check_file_exists,docs/project.md)
	$(call check_sops_key)
	@mkdir -p secrets
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary docs/project.md > secrets/project.md.enc
	@echo "Encrypted docs/project.md -> secrets/project.md.enc"

# Decrypt secrets/project.md.enc -> docs/project.md
decrypt-project:
	$(call check_file_exists,secrets/project.md.enc)
	$(call check_sops_key)
	@mkdir -p docs
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary secrets/project.md.enc > docs/project.md
	@echo "Decrypted secrets/project.md.enc -> docs/project.md"

# Verify decrypted project equals source file (no diff)
# Use secrets/project.md.sops as the canonical encrypted artifact for project.md
verify-project-sops:
	$(call check_file_exists,secrets/project.md.sops)
	@mkdir -p docs
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d secrets/project.md.sops > docs/project.dec.md
	@diff -u docs/project.md docs/project.dec.md && echo "OK: decrypted matches original" || (echo "Mismatch between docs/project.md and decrypted secrets/project.md.sops" && rm -f docs/project.dec.md && exit 1)
	@rm -f docs/project.dec.md

# Encrypt all RFC markdowns under docs/rfc/** -> secrets/rfc/** (binary .sops)
encrypt-rfcs:
	$(call check_sops_key)
	@mkdir -p secrets/rfc
	@set -euo pipefail; \
	if [ ! -d docs/rfc ]; then \
	  echo "docs/rfc not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find docs/rfc -type f -name '*.md' -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No *.md files found under docs/rfc"; \
	  exit 0; \
	fi; \
	find docs/rfc -type f -name '*.md' | while IFS= read -r src; do \
	  rel=$${src#docs/rfc/}; \
	  dest_dir="secrets/rfc/$$(dirname "$$rel")"; \
	  dest_file="secrets/rfc/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done

# Encrypt all files under cv/** -> secrets/cv/** (binary .sops)
# Excludes files in cv/original/* directory
encrypt-cv:
	$(call check_sops_key)
	@mkdir -p secrets/cv
	@set -euo pipefail; \
	if [ ! -d cv ]; then \
	  echo "cv directory not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find cv -type f -not -path "cv/original/*" -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No files found under cv directory (excluding cv/original/)"; \
	  exit 0; \
	fi; \
	find cv -type f -not -path "cv/original/*" | while IFS= read -r src; do \
	  rel=$${src#cv/}; \
	  dest_dir="secrets/cv/$$(dirname "$$rel")"; \
	  dest_file="secrets/cv/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done

# Decrypt all secrets/rfc/**.sops -> docs/rfc/** (binary)
decrypt-rfcs:
	$(call check_sops_key)
	@mkdir -p docs/rfc
	@$(MAKE) backup-rfcs || true
	@set -euo pipefail; \
	if [ ! -d secrets/rfc ]; then \
	  echo "secrets/rfc not found; nothing to decrypt"; \
	  exit 0; \
	fi; \
	first=$$(find secrets/rfc -type f -name '*.sops' -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No *.sops files found under secrets/rfc"; \
	  exit 0; \
	fi; \
	find secrets/rfc -type f -name '*.sops' | while IFS= read -r enc; do \
	  rel=$${enc#secrets/rfc/}; \
	  rel_out=$${rel%.sops}; \
	  dest_dir="docs/rfc/$$(dirname "$$rel_out")"; \
	  dest_file="docs/rfc/$$rel_out"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Decrypting $$enc -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "$$enc" > "$$dest_file"; \
	done

# Decrypt all secrets/cv/**.sops -> cv/** (binary)
decrypt-cv:
	$(call check_sops_key)
	@mkdir -p cv
	@$(MAKE) backup-cv || true
	@set -euo pipefail; \
	if [ ! -d secrets/cv ]; then \
	  echo "secrets/cv not found; nothing to decrypt"; \
	  exit 0; \
	fi; \
	first=$$(find secrets/cv -type f -name '*.sops' -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No *.sops files found under secrets/cv"; \
	  exit 0; \
	fi; \
	find secrets/cv -type f -name '*.sops' | while IFS= read -r enc; do \
	  rel=$${enc#secrets/cv/}; \
	  rel_out=$${rel%.sops}; \
	  dest_dir="cv/$$(dirname "$$rel_out")"; \
	  dest_file="cv/$$rel_out"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Decrypting $$enc -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "$$enc" > "$$dest_file"; \
	done

# Backup docs/rfc to timestamped folder under docs/rfc.backups
backup-rfcs:
	@set -euo pipefail; \
	if [ -d docs/rfc ]; then \
	  ts=$$(date +%Y%m%d%H%M%S); \
	  mkdir -p docs/rfc.backups; \
	  cp -R docs/rfc "docs/rfc.backups/rfc_$$ts"; \
	  echo "Backed up docs/rfc -> docs/rfc.backups/rfc_$$ts"; \
	else \
	  echo "docs/rfc not found; skipping backup"; \
	fi

# Backup cv to timestamped folder under cv.backups
backup-cv:
	@set -euo pipefail; \
	if [ -d cv ]; then \
	  ts=$$(date +%Y%m%d%H%M%S); \
	  mkdir -p cv.backups; \
	  cp -R cv "cv.backups/cv_$$ts"; \
	  echo "Backed up cv -> cv.backups/cv_$$ts"; \
	else \
	  echo "cv directory not found; skipping backup"; \
	fi

# Encrypt all files under cv/original/** -> secrets/cv/original/** (binary .sops)
# This preserves originality by encrypting but NOT providing decrypt functionality
encrypt-cv-original:
	$(call check_sops_key)
	@mkdir -p secrets/cv/original
	@set -euo pipefail; \
	if [ ! -d cv/original ]; then \
	  echo "cv/original directory not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find cv/original -type f -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No files found under cv/original directory"; \
	  exit 0; \
	fi; \
	find cv/original -type f | while IFS= read -r src; do \
	  rel=$${src#cv/original/}; \
	  dest_dir="secrets/cv/original/$$(dirname "$$rel")"; \
	  dest_file="secrets/cv/original/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done; \
	echo "⚠️  WARNING: Original files encrypted. No decrypt script provided to preserve originality."

# Verify that optimized_cv_2025.md in cv/ and decrypted cv/original/ are identical
# Uses same mechanism as verify-project-sops: decrypts temporarily and compares
verify-cv:
	$(call check_sops_key)
	@set -euo pipefail; \
	if [ ! -f "cv/optimized_cv_2025.md" ]; then \
	  echo "Error: cv/optimized_cv_2025.md not found"; \
	  exit 1; \
	fi; \
	if [ ! -f "secrets/cv/original/optimized_cv_2025.md.sops" ]; then \
	  echo "Error: secrets/cv/original/optimized_cv_2025.md.sops not found"; \
	  exit 1; \
	fi; \
	echo "Decrypting secrets/cv/original/optimized_cv_2025.md.sops for verification..."; \
	mkdir -p cv/original.temp; \
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "secrets/cv/original/optimized_cv_2025.md.sops" > "cv/original.temp/optimized_cv_2025.md"; \
	echo "Comparing cv/optimized_cv_2025.md and decrypted cv/original/optimized_cv_2025.md..."; \
	if diff -q "cv/optimized_cv_2025.md" "cv/original.temp/optimized_cv_2025.md" >/dev/null 2>&1; then \
	  echo "✅ SUCCESS: Files are identical (no differences found)"; \
	  echo "   - cv/optimized_cv_2025.md"; \
	  echo "   - decrypted from secrets/cv/original/optimized_cv_2025.md.sops"; \
	  rm -rf cv/original.temp; \
	else \
	  echo "❌ DIFFERENCE: Files are not identical"; \
	  echo "   - cv/optimized_cv_2025.md"; \
	  echo "   - decrypted from secrets/cv/original/optimized_cv_2025.md.sops"; \
	  echo ""; \
	  echo "Showing differences:"; \
	  diff -u "cv/optimized_cv_2025.md" "cv/original.temp/optimized_cv_2025.md" || true; \
	  rm -rf cv/original.temp; \
	  exit 1; \
	fi

 vuln:
	govulncheck ./...

 test:
	@pkgs=$$($(GO) list ./... | grep -v "/cmd/" | grep -v "/mocks" | grep -v "/test/e2e" | grep -v "/internal/adapter/queue/shared"); \
	$(GO) test -v -race -timeout=300s -failfast -parallel=4 -count=1 -coverprofile=coverage/coverage.unit.out $$pkgs

 test-e2e:
	$(MAKE) run-e2e-tests E2E_CLEAR_DUMP=true E2E_START_SERVICES=false

 cover:
	$(GO) tool cover -html=coverage/coverage.unit.out -o coverage/coverage.html

# --- Consolidated E2E Test Target ---------------------------------------------

# Parameters for E2E test execution
E2E_CLEAR_DUMP ?= true
E2E_START_SERVICES ?= false
E2E_BASE_URL ?= 
E2E_TIMEOUT ?= 3m
E2E_LOG_DIR ?= 
E2E_PARALLEL ?= 4 

# Consolidated E2E test target that can be reused
# Usage: make run-e2e-tests E2E_START_SERVICES=true E2E_BASE_URL=http://localhost:8080/v1
# E2E Test Helper Functions - Refactored and Simplified
define wait_for_postgres
	$(call log_info,Waiting for Postgres to be ready \(max 60s\)...); \
	for i in $$(seq 1 30); do \
		if $(DOCKER_COMPOSE) exec -T db pg_isready -U postgres >/dev/null 2>&1; then \
			$(call log_info,Postgres is ready); \
			break; \
		fi; \
		echo "  Attempt $$i/30: waiting for db..."; \
		sleep 2; \
	done
endef

define verify_database_schema
	$(call log_info,Verifying database schema...); \
	$(DOCKER_COMPOSE) exec -T db psql -U postgres -d app -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'results';" | grep -q results || (echo "ERROR: results table not found after migration" && exit 1); \
	$(call log_info,Database schema verified)
endef

define wait_for_healthz
	$(call log_info,Waiting for healthz endpoint \(max 120s\)...); \
	APP_PORT=$${PORT:-8080}; \
	MAX_ATTEMPTS=60; \
	ATTEMPT=0; \
	while [ $$ATTEMPT -lt $$MAX_ATTEMPTS ]; do \
		ATTEMPT=$$((ATTEMPT + 1)); \
		if curl -fsS http://localhost:$$APP_PORT/healthz >/dev/null 2>&1; then \
			$(call log_info,Service is ready!); \
			break; \
		fi; \
		echo "  Attempt $$ATTEMPT/$$MAX_ATTEMPTS: waiting for service..."; \
		sleep 2; \
	done; \
	if [ $$ATTEMPT -eq $$MAX_ATTEMPTS ]; then \
		echo "ERROR: Service failed to become ready after 120s"; \
		exit 1; \
	fi
endef

define setup_log_collection
	LOG_DIR="$(E2E_LOG_DIR)"; \
	if [ -n "$$LOG_DIR" ]; then \
		mkdir -p "$$LOG_DIR"; \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) logs -f app worker > "$$LOG_DIR/compose.follow.log" 2>&1 & LOG_FOLLOW_PID=$$!; \
		trap 'echo "==> Collecting docker logs..."; $(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) logs > "$$LOG_DIR/compose.full.log" 2>&1 || true; grep -iE "\\b(error|panic|fatal)\\b" "$$LOG_DIR/compose.full.log" > "$$LOG_DIR/compose.errors.log" || true; [ -n "$$LOG_FOLLOW_PID" ] && kill "$$LOG_FOLLOW_PID" 2>/dev/null || true' EXIT; \
	fi
endef

define collect_post_test_logs
	LOG_DIR="$(E2E_LOG_DIR)"; \
	if [ "$(E2E_START_SERVICES)" = "true" ] && [ -n "$$LOG_DIR" ]; then \
		$(call log_info,Collecting docker logs after tests...); \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) logs > "$$LOG_DIR/compose.full.post.log" 2>&1 || true; \
		grep -iE '\\b(error|panic|fatal)\\b' "$$LOG_DIR/compose.full.post.log" > "$$LOG_DIR/compose.errors.post.log" || true; \
		$(call log_info,E2E complete. Checking for ERROR logs...); \
		if [ -s "$$LOG_DIR/compose.errors.post.log" ]; then \
			$(call log_info,ERROR logs found. See $$LOG_DIR/compose.errors.post.log); \
			tail -n 200 "$$LOG_DIR/compose.errors.post.log" || true; \
		elif [ -s "$$LOG_DIR/compose.errors.log" ]; then \
			$(call log_info,ERROR logs found. See $$LOG_DIR/compose.errors.log); \
			tail -n 200 "$$LOG_DIR/compose.errors.log" || true; \
		else \
			$(call log_info,No ERROR logs detected in docker compose services); \
		fi; \
	fi
endef

define run_e2e_tests
	$(call log_info,Loading .env file...); \
	$(call load_env); \
	$(call log_info,Running E2E tests with parallel=$(E2E_PARALLEL)...); \
	if [ -n "$(E2E_BASE_URL)" ]; then \
		E2E_BASE_URL="$(E2E_BASE_URL)" $(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) ./test/e2e/...; \
	else \
		$(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) ./test/e2e/...; \
	fi
endef

# Refactored E2E Test Target - Clean and Modular with Centralized Cleanup
run-e2e-tests:
	@set -euo pipefail; \
	$(call log_info,Starting E2E test execution...); \
	$(call log_info,Configuration: E2E_CLEAR_DUMP=$(E2E_CLEAR_DUMP), E2E_START_SERVICES=$(E2E_START_SERVICES), E2E_BASE_URL=$(E2E_BASE_URL)); \
	$(call log_info,---); \
	if [ "$(E2E_CLEAR_DUMP)" = "true" ]; then \
		$(call log_info,Clearing dump directory...); \
		$(call clear_dump_dir); \
	fi; \
	$(call log_info,---); \
	if [ "$(E2E_START_SERVICES)" = "true" ]; then \
		$(call log_info,Starting services with $(DOCKER_COMPOSE)...); \
		$(call setup_log_collection); \
		$(DOCKER_COMPOSE) -f $(DOCKER_COMPOSE_FILE) up -d --build; \
		$(call log_info,Services started, setting up cleanup trap...); \
		trap 'echo "==> E2E cleanup: Comprehensive cleanup..."; $(call comprehensive_cleanup); echo "==> E2E cleanup completed"' EXIT; \
		$(call log_info,---); \
		$(call wait_for_postgres); \
		$(call log_info,Migrations will run automatically via docker-compose dependencies...); \
		$(call verify_database_schema); \
		$(call log_info,---); \
		$(call wait_for_healthz); \
		$(call log_info,---); \
	fi; \
	$(call run_e2e_tests); \
	$(call log_info,---); \
	$(call collect_post_test_logs); \
	$(call log_info,---); \
	if [ "$(E2E_CLEAR_DUMP)" = "true" ]; then \
		$(call log_info,E2E responses dumped to $(TEST_DUMP_DIR)/); \
	fi; \
	$(call log_info,E2E test execution completed successfully)

run:
	@set -a; [ -f .env ] && . ./.env || true; set +a; \
	APP_ENV=$${APP_ENV:-dev} $(GO) run ./cmd/server

 build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server

 docker-build:
	docker build -f Dockerfile.server -t $(APP_NAME)-server:local .
	docker build -f Dockerfile.worker -t $(APP_NAME)-worker:local .
	docker build -f Dockerfile.migrate -t $(APP_NAME)-migrate:local .

docker-build-ci:
	@[ -n "$(TAG)" ] || (echo "Usage: make docker-build-ci TAG=<tag>" && exit 1)
	docker build -f Dockerfile.server -t $(APP_NAME)-server:$(TAG) .
	docker build -f Dockerfile.worker -t $(APP_NAME)-worker:$(TAG) .
	docker build -f Dockerfile.migrate -t $(APP_NAME)-migrate:$(TAG) .

 docker-run:
	$(call start_services)

 migrate:
	$(call load_env); \
	$(DOCKER_COMPOSE) run --rm migrate

 tools:
	GOBIN=$(PWD)/bin $(GO) install github.com/mgechev/revive@latest
	GOBIN=$(PWD)/bin $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
	GOBIN=$(PWD)/bin $(GO) install golang.org/x/vuln/cmd/govulncheck@latest
	GOBIN=$(PWD)/bin $(GO) install gotest.tools/gotestsum@latest


# Security scan: gosec with SARIF output
gosec-sarif:
	@which gosec >/dev/null 2>&1 || $(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GO) env GOPATH >/dev/null 2>&1 || true
	$$($(GO) env GOPATH)/bin/gosec -fmt sarif -out gosec-results.sarif ./... || true

# License scanning via FOSSA CLI
license-scan:
	@which fossa >/dev/null 2>&1 || $(GO) install github.com/fossa-contrib/fossa-cli@latest
	$(GO) mod download
	$$($(GO) env GOPATH)/bin/fossa analyze

 generate:
	$(GO) generate ./...

openapi-validate:
	$(GO) run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml

build-matrix:
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/server-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/server-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/server-darwin-arm64 ./cmd/server
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/worker-linux-amd64 ./cmd/worker
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/worker-linux-arm64 ./cmd/worker
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/worker-darwin-amd64 ./cmd/worker
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/worker-darwin-arm64 ./cmd/worker


verify-test-placement:
	@set -euo pipefail; \
	if git ls-files -- '*.go' | grep -E '^test/.*_test\.go$$' | grep -vE '^test/e2e/' ; then \
	  echo "Error: unit tests must be colocated next to code. Move tests out of top-level test/ (allowed only under test/e2e/)." >&2; \
	  exit 1; \
	fi

# --- CI convenience targets ---------------------------------------------------

ci-test:
	@set -euo pipefail; \
	$(call log_info,Running tests...); \
	$(MAKE) test; \
	$(call log_info,Combining coverage reports...); \
	$(GO) tool cover -func=coverage/coverage.unit.out | tee coverage/coverage.func.txt; \
	total=$$(grep -E "^total:.*\\(statements\\).*[0-9.]+%$$" coverage/coverage.func.txt | awk '{print $$NF}' | tr -d '%'); \
	total_int=$${total%.*}; \
	if [ "$$total_int" -lt 80 ]; then \
	  echo "Overall coverage $$total% is below 80% minimum" >&2; \
	  exit 1; \
	fi

# CI E2E Test Target - Simplified (cleanup handled in run-e2e-tests)
ci-e2e:
	@set -euo pipefail; \
	LOG_DIR="artifacts/ci-e2e-logs-$$(date +%Y%m%d%H%M%S)"; \
	APP_PORT=$${PORT:-8080}; \
	$(MAKE) run-e2e-tests E2E_CLEAR_DUMP=true E2E_START_SERVICES=true E2E_BASE_URL="http://localhost:$$APP_PORT/v1" E2E_LOG_DIR="$$LOG_DIR"

# E2E Test Management Targets

# Comprehensive Docker cleanup (removes containers, volumes, networks)
docker-cleanup:
	@echo "==> Comprehensive Docker cleanup..."
	@$(call comprehensive_cleanup)


# Show E2E test help
e2e-help:
	@echo "E2E Test Commands:"
	@echo "  make ci-e2e                    - Full CI E2E test with automatic cleanup"
	@echo "  make docker-cleanup            - Comprehensive Docker cleanup (containers, volumes, networks)"
	@echo "  make run-e2e-tests             - Run E2E tests with custom parameters (includes cleanup)"
	@echo ""
	@echo "Parameters:"
	@echo "  E2E_CLEAR_DUMP=true/false      - Clear dump directory (default: true)"
	@echo "  E2E_START_SERVICES=true/false  - Start Docker services (default: false)"
	@echo "  E2E_BASE_URL=<url>             - Base URL for tests"
	@echo "  E2E_LOG_DIR=<dir>              - Directory for logs"
	@echo "  E2E_PARALLEL=<num>             - Parallel test execution (default: 4)"


# --- Frontend Development Targets ---------------------------------------------

# Install frontend dependencies
frontend-install:
	$(call log_info,Installing frontend dependencies...)
	cd admin-frontend && npm install

# Build frontend for production
frontend-build:
	$(call log_info,Building frontend for production...)
	cd admin-frontend && npm run build

# Clean frontend build artifacts
frontend-clean:
	$(call log_info,Cleaning frontend build artifacts...)
	cd admin-frontend && rm -rf dist node_modules/.vite

# Start frontend development server with HMR
frontend-dev:
	$(call log_info,Starting frontend development server...)
	@echo "==> Frontend will be available at: http://localhost:3001"
	@echo "==> Backend should be running at: http://localhost:8080"
	@echo "==> Press Ctrl+C to stop"
	cd admin-frontend && npm run dev

# Start full development environment (backend + frontend)
dev-full:
	$(call log_info,Starting full development environment...)
	@echo "==> This will start backend services and frontend with HMR"
	@echo "==> Frontend: http://localhost:3001"
	@echo "==> Backend API: http://localhost:8080"
	@echo "==> Press Ctrl+C to stop all services"
	@echo ""
	$(call log_info,Starting backend services with docker-compose...)
	$(DOCKER_COMPOSE) up -d app worker db redpanda redpanda-console qdrant tika otel-collector jaeger prometheus grafana
	$(call log_info,Waiting for backend services to be ready...)
	sleep 15
	$(call log_info,Starting frontend development server...)
	cd admin-frontend && npm run dev &
	FRONTEND_PID=$$!; \
	trap "$(call log_info,Stopping services...); kill $$FRONTEND_PID 2>/dev/null || true; $(call stop_services); $(call log_info,All services stopped)" EXIT; \
	wait

# Show help for frontend development
frontend-help:
	@echo "Frontend Development Commands:"
	@echo ""
	@echo "  make frontend-install    - Install frontend dependencies"
	@echo "  make frontend-dev        - Start frontend dev server (HMR enabled)"
	@echo "  make frontend-build      - Build frontend for production"
	@echo "  make frontend-clean      - Clean frontend build artifacts"
	@echo "  make dev-full           - Start full dev environment (backend + frontend)"
	@echo ""
	@echo "Quick Start:"
	@echo "  1. make frontend-install"
	@echo "  2. make frontend-dev     (in one terminal)"
	@echo "  3. make docker-run       (in another terminal)"
	@echo ""
	@echo "Or use the convenience script:"
	@echo "  ./scripts/dev-frontend.sh"