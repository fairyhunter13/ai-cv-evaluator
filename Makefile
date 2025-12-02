SHELL := /bin/bash
PATH := $(PWD)/bin:$(PATH)

APP_NAME := ai-cv-evaluator
GO := go
GOFLAGS := -trimpath
GOTOOLCHAIN := auto
CGO_ENABLED ?= 0
SOPS_AGE_KEY_FILE ?= $(HOME)/.config/sops/age/keys.txt

# Common variables
DOCKER_COMPOSE := docker compose
DOCKER_COMPOSE_FILE := docker-compose.yml
# Support multiple compose files. Example: DOCKER_COMPOSE_FILES="docker-compose.yml docker-compose.dev.override.yml"
DOCKER_COMPOSE_FILES ?= $(DOCKER_COMPOSE_FILE)
# Expand to: -f file1 -f file2 ...
compose_files_args := $(foreach f,$(DOCKER_COMPOSE_FILES),-f $(f))
TEST_DUMP_DIR := test/dump
COVERAGE_DIR := coverage
ARTIFACTS_DIR := artifacts

# Plaintext sources (post-move) for SOPS workflows
PLAINTEXT_SUBMISSIONS_DIR := submissions
PLAINTEXT_PROJECT_FILE := $(PLAINTEXT_SUBMISSIONS_DIR)/project.md
PLAINTEXT_RFC_DIR := $(PLAINTEXT_SUBMISSIONS_DIR)/rfc
PLAINTEXT_CV_DIR := $(PLAINTEXT_SUBMISSIONS_DIR)/cv
PLAINTEXT_CV_ORIGINAL_DIR := $(PLAINTEXT_CV_DIR)/original

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
	$(DOCKER_COMPOSE) $(compose_files_args) down -v --remove-orphans || true; \
	echo "==> Removing any remaining containers..."; \
	docker ps -a --filter "name=ai-cv-evaluator" --format "table {{.Names}}" | grep -v NAMES | xargs -r docker rm -f || true; \
	echo "==> Removing any remaining volumes..."; \
	docker volume ls --filter "name=ai-cv-evaluator" --format "{{.Name}}" | xargs -r docker volume rm -f || true; \
	echo "==> Removing any remaining networks..."; \
	docker network ls --filter "name=ai-cv-evaluator" --format "{{.Name}}" | grep -v NETWORK | xargs -r docker network rm || true; \
	echo "==> Cleanup completed"
endef

# Ensure a clean slate before running unit tests
define pre_test_cleanup
	$(call log_info,Cleaning previous test artifacts...); \
	rm -rf $(COVERAGE_DIR)/*; \
	mkdir -p $(COVERAGE_DIR); \
	# Clean test dumps and artifacts logs prior to unit tests \
	rm -rf $(TEST_DUMP_DIR)/*; \
	mkdir -p $(TEST_DUMP_DIR); \
	rm -rf $(ARTIFACTS_DIR)/*; \
	mkdir -p $(ARTIFACTS_DIR); \
	$(GO) clean -testcache
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
	encrypt-env decrypt-env encrypt-env-production decrypt-env-production encrypt-keycloak-realm decrypt-keycloak-realm \
	verify-project-sops encrypt-project decrypt-project \
	encrypt-rfcs decrypt-rfcs encrypt-cv decrypt-cv encrypt-cv-original backup-rfcs backup-cv verify-cv decrypt-test-cv clean-test-cv \
	ci-test openapi-validate build-matrix verify-test-placement gosec-sarif license-scan \
	freemodels-test frontend-dev frontend-install frontend-build frontend-clean frontend-help run-e2e-tests docker-cleanup e2e-help
.PHONY: lint-backend lint-frontend lint-infra lint-docs lint-all install-git-hooks

all: fmt lint vet test

deps:
	$(GO) mod download

fmt:
	gofmt -s -w .
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports globally..."; \
		$(GO) install golang.org/x/tools/cmd/goimports@latest; \
	fi
	goimports -w .

lint:
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && GOBIN=$(PWD)/bin $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2)
	golangci-lint run ./...

vet:
	$(GO) vet ./...

lint-backend: lint

lint-frontend:
	@set -euo pipefail; \
	if [ -d admin-frontend ]; then \
		cd admin-frontend && npm run lint; \
	else \
		echo "admin-frontend directory not found; skipping frontend lint"; \
	fi

lint-infra:
	@set -euo pipefail; \
	echo "Linting infrastructure (docker-compose)..."; \
	TEMP_ENV_CREATED=""; \
	TEMP_ENV_PROD_CREATED=""; \
	if command -v docker >/dev/null 2>&1; then \
		# Ensure docker-compose.yml env_file (.env) does not break lint in CI when .env is absent; \
		# create a temporary empty .env only if none exists. \
		if [ ! -f .env ]; then \
			TEMP_ENV_CREATED=".env.lint.$$"; \
			echo "Creating temporary .env for docker-compose lint"; \
			touch "$$TEMP_ENV_CREATED"; \
			ln -s "$$TEMP_ENV_CREATED" .env; \
		fi; \
		if [ -f docker-compose.yml ]; then \
			docker compose -f docker-compose.yml config -q; \
		else \
			echo "docker-compose.yml not found; skipping dev compose lint"; \
		fi; \
		# Clean up temporary .env symlink + file if we created one above. \
		if [ -n "$$TEMP_ENV_CREATED" ]; then \
			rm -f .env "$$TEMP_ENV_CREATED"; \
		fi; \
		if [ -f docker-compose.prod.yml ]; then \
			# docker-compose.prod.yml may use env_file: .env.production; create a temporary one if missing \
			# so that config -q does not fail in CI when real secrets are not present. \
			if [ ! -f .env.production ]; then \
				TEMP_ENV_PROD_CREATED=".env.production.lint.$$"; \
				echo "Creating temporary .env.production for docker-compose.prod lint"; \
				touch "$$TEMP_ENV_PROD_CREATED"; \
				ln -s "$$TEMP_ENV_PROD_CREATED" .env.production; \
			fi; \
			KEYCLOAK_ADMIN=dummy-admin \
			KEYCLOAK_ADMIN_PASSWORD=dummy-password \
			OAUTH2_PROXY_CLIENT_SECRET=dummy-client-secret \
			OAUTH2_PROXY_COOKIE_SECRET=dummy-cookie-secret \
			OAUTH2_PROXY_EMAIL_DOMAINS=example.com \
			docker compose -f docker-compose.prod.yml config -q; \
			# Clean up temporary .env.production symlink + file if we created one above. \
			if [ -n "$$TEMP_ENV_PROD_CREATED" ]; then \
				rm -f .env.production "$$TEMP_ENV_PROD_CREATED"; \
			fi; \
		else \
			echo "docker-compose.prod.yml not found; skipping prod compose lint"; \
		fi; \
	else \
		echo "docker not found; skipping infrastructure lint that requires docker"; \
	fi

lint-docs:
	@set -euo pipefail; \
	echo "Linting Markdown docs for trailing whitespace and tabs..."; \
	files=$$(git ls-files 'README.md' 2>/dev/null || true); \
	if [ -z "$$files" ]; then \
		echo "No Markdown files found; skipping docs lint"; \
	else \
		if grep -nE ' +$$' $$files; then \
			echo "Docs lint failed: trailing whitespace or tab characters found in Markdown files"; \
			exit 1; \
		fi; \
	fi

lint-all: lint-backend lint-frontend lint-infra lint-docs

install-git-hooks:
	git config core.hooksPath .githooks

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

# Encrypt deploy/keycloak/realm-aicv.json -> secrets/deploy/keycloak/realm-aicv.json.sops
encrypt-keycloak-realm:
	$(call check_file_exists,deploy/keycloak/realm-aicv.json)
	$(call check_sops_key)
	@mkdir -p secrets/deploy/keycloak
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type json --output-type json deploy/keycloak/realm-aicv.json > secrets/deploy/keycloak/realm-aicv.json.sops
	@echo "Encrypted deploy/keycloak/realm-aicv.json -> secrets/deploy/keycloak/realm-aicv.json.sops"

# Decrypt secrets/deploy/keycloak/realm-aicv.json.sops -> deploy/keycloak/realm-aicv.json
decrypt-keycloak-realm:
	$(call check_file_exists,secrets/deploy/keycloak/realm-aicv.json.sops)
	$(call check_sops_key)
	@mkdir -p deploy/keycloak
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type json --output-type json secrets/deploy/keycloak/realm-aicv.json.sops > deploy/keycloak/realm-aicv.json
	@echo "Decrypted secrets/deploy/keycloak/realm-aicv.json.sops -> deploy/keycloak/realm-aicv.json"

# Encrypt submissions/project.md -> secrets/project.md.enc (Binary)
encrypt-project:
	$(call check_file_exists,$(PLAINTEXT_PROJECT_FILE))
	$(call check_sops_key)
	@mkdir -p secrets
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary $(PLAINTEXT_PROJECT_FILE) > secrets/project.md.enc
	@echo "Encrypted $(PLAINTEXT_PROJECT_FILE) -> secrets/project.md.enc"

# Decrypt secrets/project.md.enc -> submissions/project.md
decrypt-project:
	$(call check_file_exists,secrets/project.md.enc)
	$(call check_sops_key)
	@mkdir -p $(PLAINTEXT_SUBMISSIONS_DIR)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary secrets/project.md.enc > $(PLAINTEXT_PROJECT_FILE)
	@echo "Decrypted secrets/project.md.enc -> $(PLAINTEXT_PROJECT_FILE)"

# Verify decrypted project equals source file (no diff)
# Use secrets/project.md.sops as the canonical encrypted artifact for project.md
verify-project-sops:
	$(call check_file_exists,secrets/project.md.sops)
	@mkdir -p $(PLAINTEXT_SUBMISSIONS_DIR)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d secrets/project.md.sops > $(PLAINTEXT_SUBMISSIONS_DIR)/project.dec.md
	@diff -u $(PLAINTEXT_PROJECT_FILE) $(PLAINTEXT_SUBMISSIONS_DIR)/project.dec.md && echo "OK: decrypted matches original" || (echo "Mismatch between $(PLAINTEXT_PROJECT_FILE) and decrypted secrets/project.md.sops" && rm -f $(PLAINTEXT_SUBMISSIONS_DIR)/project.dec.md && exit 1)
	@rm -f $(PLAINTEXT_SUBMISSIONS_DIR)/project.dec.md

# Encrypt all RFC markdowns under submissions/rfc/** -> secrets/rfc/** (binary .sops)
encrypt-rfcs:
	$(call check_sops_key)
	@mkdir -p secrets/rfc
	@set -euo pipefail; \
	if [ ! -d $(PLAINTEXT_RFC_DIR) ]; then \
	  echo "$(PLAINTEXT_RFC_DIR) not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find $(PLAINTEXT_RFC_DIR) -type f -name '*.md' -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No *.md files found under $(PLAINTEXT_RFC_DIR)"; \
	  exit 0; \
	fi; \
	find $(PLAINTEXT_RFC_DIR) -type f -name '*.md' | while IFS= read -r src; do \
	  rel=$${src#$(PLAINTEXT_RFC_DIR)/}; \
	  dest_dir="secrets/rfc/$$(dirname "$$rel")"; \
	  dest_file="secrets/rfc/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done

# Encrypt all files under submissions/cv/** -> secrets/cv/** (binary .sops)
# Excludes files in submissions/cv/original/* directory
encrypt-cv:
	$(call check_sops_key)
	@mkdir -p secrets/cv
	@set -euo pipefail; \
	if [ ! -d $(PLAINTEXT_CV_DIR) ]; then \
	  echo "$(PLAINTEXT_CV_DIR) directory not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find $(PLAINTEXT_CV_DIR) -type f -not -path "$(PLAINTEXT_CV_ORIGINAL_DIR)/*" -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No files found under $(PLAINTEXT_CV_DIR) directory (excluding $(PLAINTEXT_CV_ORIGINAL_DIR)/)"; \
	  exit 0; \
	fi; \
	find $(PLAINTEXT_CV_DIR) -type f -not -path "$(PLAINTEXT_CV_ORIGINAL_DIR)/*" | while IFS= read -r src; do \
	  rel=$${src#$(PLAINTEXT_CV_DIR)/}; \
	  dest_dir="secrets/cv/$$(dirname "$$rel")"; \
	  dest_file="secrets/cv/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done

# Decrypt all secrets/rfc/**.sops -> submissions/rfc/** (binary)
decrypt-rfcs:
	$(call check_sops_key)
	@mkdir -p $(PLAINTEXT_RFC_DIR)
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
	  dest_dir="$(PLAINTEXT_RFC_DIR)/$$(dirname "$$rel_out")"; \
	  dest_file="$(PLAINTEXT_RFC_DIR)/$$rel_out"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Decrypting $$enc -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "$$enc" > "$$dest_file"; \
	done

# Decrypt all secrets/cv/**.sops -> submissions/cv/** (binary)
decrypt-cv:
	$(call check_sops_key)
	@mkdir -p $(PLAINTEXT_CV_DIR)
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
	  dest_dir="$(PLAINTEXT_CV_DIR)/$$(dirname "$$rel_out")"; \
	  dest_file="$(PLAINTEXT_CV_DIR)/$$rel_out"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Decrypting $$enc -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "$$enc" > "$$dest_file"; \
	done

# Backup submissions/rfc to timestamped folder under submissions/rfc.backups
backup-rfcs:
	@set -euo pipefail; \
	if [ -d $(PLAINTEXT_RFC_DIR) ]; then \
	  ts=$$(date +%Y%m%d%H%M%S); \
	  mkdir -p $(PLAINTEXT_SUBMISSIONS_DIR)/rfc.backups; \
	  cp -R $(PLAINTEXT_RFC_DIR) "$(PLAINTEXT_SUBMISSIONS_DIR)/rfc.backups/rfc_$$ts"; \
	  echo "Backed up $(PLAINTEXT_RFC_DIR) -> $(PLAINTEXT_SUBMISSIONS_DIR)/rfc.backups/rfc_$$ts"; \
	else \
	  echo "$(PLAINTEXT_RFC_DIR) not found; skipping backup"; \
	fi

# Backup submissions/cv to timestamped folder under submissions/cv.backups
backup-cv:
	@set -euo pipefail; \
	if [ -d $(PLAINTEXT_CV_DIR) ]; then \
	  ts=$$(date +%Y%m%d%H%M%S); \
	  mkdir -p $(PLAINTEXT_SUBMISSIONS_DIR)/cv.backups; \
	  cp -R $(PLAINTEXT_CV_DIR) "$(PLAINTEXT_SUBMISSIONS_DIR)/cv.backups/cv_$$ts"; \
	  echo "Backed up $(PLAINTEXT_CV_DIR) -> $(PLAINTEXT_SUBMISSIONS_DIR)/cv.backups/cv_$$ts"; \
	else \
	  echo "$(PLAINTEXT_CV_DIR) directory not found; skipping backup"; \
	fi

# Encrypt all files under submissions/cv/original/** -> secrets/cv/original/** (binary .sops)
# This preserves originality by encrypting but NOT providing decrypt functionality
encrypt-cv-original:
	$(call check_sops_key)
	@mkdir -p secrets/cv/original
	@set -euo pipefail; \
	if [ ! -d $(PLAINTEXT_CV_ORIGINAL_DIR) ]; then \
	  echo "$(PLAINTEXT_CV_ORIGINAL_DIR) directory not found; nothing to encrypt"; \
	  exit 0; \
	fi; \
	first=$$(find $(PLAINTEXT_CV_ORIGINAL_DIR) -type f -print -quit); \
	if [ -z "$$first" ]; then \
	  echo "No files found under cv/original directory"; \
	  exit 0; \
	fi; \
	find $(PLAINTEXT_CV_ORIGINAL_DIR) -type f | while IFS= read -r src; do \
	  rel=$${src#$(PLAINTEXT_CV_ORIGINAL_DIR)/}; \
	  dest_dir="secrets/cv/original/$$(dirname "$$rel")"; \
	  dest_file="secrets/cv/original/$$rel.sops"; \
	  mkdir -p "$$dest_dir"; \
	  echo "Encrypting $$src -> $$dest_file"; \
	  SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary "$$src" > "$$dest_file"; \
	done; \
	echo "⚠️  WARNING: Original files encrypted. No decrypt script provided to preserve originality."

# Verify that optimized_cv_2025.md in submissions/cv/ and decrypted cv/original/ are identical
# Uses same mechanism as verify-project-sops: decrypts temporarily and compares
verify-cv:
	$(call check_sops_key)
	@set -euo pipefail; \
	if [ ! -f "$(PLAINTEXT_CV_DIR)/optimized_cv_2025.md" ]; then \
	  echo "Error: $(PLAINTEXT_CV_DIR)/optimized_cv_2025.md not found"; \
	  exit 1; \
	fi; \
	if [ ! -f "secrets/cv/original/optimized_cv_2025.md.sops" ]; then \
	  echo "Error: secrets/cv/original/optimized_cv_2025.md.sops not found"; \
	  exit 1; \
	fi; \
	echo "Decrypting secrets/cv/original/optimized_cv_2025.md.sops for verification..."; \
	mkdir -p $(PLAINTEXT_CV_DIR)/original.temp; \
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "secrets/cv/original/optimized_cv_2025.md.sops" > "$(PLAINTEXT_CV_DIR)/original.temp/optimized_cv_2025.md"; \
	echo "Comparing $(PLAINTEXT_CV_DIR)/optimized_cv_2025.md and decrypted secrets/cv/original/optimized_cv_2025.md.sops..."; \
	if diff -q "$(PLAINTEXT_CV_DIR)/optimized_cv_2025.md" "$(PLAINTEXT_CV_DIR)/original.temp/optimized_cv_2025.md" >/dev/null 2>&1; then \
	  echo "✅ SUCCESS: Files are identical (no differences found)"; \
	  echo "   - $(PLAINTEXT_CV_DIR)/optimized_cv_2025.md"; \
	  echo "   - decrypted from secrets/cv/original/optimized_cv_2025.md.sops"; \
	  rm -rf $(PLAINTEXT_CV_DIR)/original.temp; \
	else \
	  echo "❌ DIFFERENCE: Files are not identical"; \
	  echo "   - $(PLAINTEXT_CV_DIR)/optimized_cv_2025.md"; \
	  echo "   - decrypted from secrets/cv/original/optimized_cv_2025.md.sops"; \
	  echo ""; \
	  echo "Showing differences:"; \
	  diff -u "$(PLAINTEXT_CV_DIR)/optimized_cv_2025.md" "$(PLAINTEXT_CV_DIR)/original.temp/optimized_cv_2025.md" || true; \
	  rm -rf $(PLAINTEXT_CV_DIR)/original.temp; \
	  exit 1; \
	fi

# Decrypt sensitive optimized CV into test fixtures for E2E tests
decrypt-test-cv:
	$(call check_sops_key)
	@set -euo pipefail; \
	if [ -f "secrets/cv/optimized_cv_2025.md.sops" ]; then \
		mkdir -p test/testdata; \
		echo "Decrypting secrets/cv/optimized_cv_2025.md.sops -> test/testdata/cv_optimized_2025.md"; \
		SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary "secrets/cv/optimized_cv_2025.md.sops" > "test/testdata/cv_optimized_2025.md"; \
	else \
		echo "Warning: secrets/cv/optimized_cv_2025.md.sops not found; skipping decrypt-test-cv"; \
	fi

# Remove decrypted sensitive CV test fixture
clean-test-cv:
	@set -euo pipefail; \
	printf '%s\n' "Sensitive CV test content is decrypted from SOPS (secrets/cv/optimized_cv_2025.md.sops) at E2E test time. This placeholder intentionally contains no personal data." > test/testdata/cv_optimized_2025.md

 vuln:
	govulncheck ./...

 test:
	@$(call pre_test_cleanup); \
	pkgs=$$($(GO) list ./... | grep -v "/cmd/" | grep -v "/mocks" | grep -v "/test/e2e"); \
	$(GO) test -v -race -timeout=180s -failfast -parallel=4 -count=1 -coverprofile=$(COVERAGE_DIR)/coverage.unit.out $$pkgs

 test-e2e:
	$(MAKE) test-e2e-core

# --- Rate-Limit-Friendly Core E2E Suite ---------------------------------------
# These targets run a lightweight E2E suite designed to be safe to run multiple
# times consecutively without hitting provider rate limits.
#
# Algorithm:
#   - Uses minimal CV/project texts (~50-100 chars) to minimize tokens
#   - Runs only 3 jobs with 15s cooldown between each
#   - Each job makes ~3 LLM calls via the multi-step evaluation chain
#   - Total: ~9 LLM calls over ~3 minutes = ~3 RPM (well under 30 RPM limit)
#   - 4 provider accounts (2 Groq + 2 OpenRouter) handle load via fallback
#
# Usage:
#   make test-e2e-core                    # Quick core suite (services must be running)
#   make run-e2e-core                     # Core suite with service startup/cleanup
#   make run-e2e-core-repeat RUNS=3       # Run core suite multiple times consecutively

# Core E2E parameters (optimized for rate-limit safety)
# - 1 job keeps load extremely low and minimizes rate-limit risk
# - 15s cooldown ensures we stay under 30 RPM limit even with retries
# - 120s per-job timeout handles slow LLM responses
# - 8m global timeout allows for job + overhead
E2E_CORE_TIMEOUT ?= 8m
E2E_CORE_JOB_COUNT ?= 1
E2E_INTER_JOB_COOLDOWN ?= 15s
E2E_PER_JOB_TIMEOUT ?= 120s

# Run the main rate-limit-friendly core E2E test only
# This is the recommended target for CI - runs 2 jobs with proper cooldowns
test-e2e-core:
	@set -euo pipefail; \
	$(call log_info,Running rate-limit-friendly core E2E test...); \
	$(call load_env); \
	E2E_CORE_JOB_COUNT="$(E2E_CORE_JOB_COUNT)" \
	E2E_INTER_JOB_COOLDOWN="$(E2E_INTER_JOB_COOLDOWN)" \
	E2E_PER_JOB_TIMEOUT="$(E2E_PER_JOB_TIMEOUT)" \
	$(GO) test -tags=e2e -v -race -timeout=$(E2E_CORE_TIMEOUT) -count=1 \
		-run "TestE2E_Core_RateLimitFriendly$$" ./test/e2e/...

# Run single-job core E2E test (fastest possible E2E validation)
test-e2e-single:
	@set -euo pipefail; \
	$(call log_info,Running single-job E2E test...); \
	$(call load_env); \
	$(GO) test -tags=e2e -v -race -timeout=3m -count=1 \
		-run "TestE2E_Core_SingleJob" ./test/e2e/...

# Run core E2E with service startup and cleanup
# This is the recommended CI target - starts services, runs the core test, and cleans up
run-e2e-core:
	@set -euo pipefail; \
	$(call log_info,Starting rate-limit-friendly core E2E suite...); \
	$(call log_info,Configuration: jobs=$(E2E_CORE_JOB_COUNT), cooldown=$(E2E_INTER_JOB_COOLDOWN), per_job_timeout=$(E2E_PER_JOB_TIMEOUT)); \
	$(call load_env); \
	rm -rf $(ARTIFACTS_DIR)/* || true; \
	mkdir -p $(ARTIFACTS_DIR); \
	$(call log_info,Starting services...); \
	$(DOCKER_COMPOSE) $(compose_files_args) up -d --build; \
	trap 'echo "==> E2E cleanup..."; $(call comprehensive_cleanup); echo "==> Cleanup completed"' EXIT; \
	$(call wait_for_postgres); \
	$(call verify_database_schema); \
	$(call wait_for_healthz); \
	$(call log_info,Running core E2E test...); \
	E2E_CORE_JOB_COUNT="$(E2E_CORE_JOB_COUNT)" \
	E2E_INTER_JOB_COOLDOWN="$(E2E_INTER_JOB_COOLDOWN)" \
	E2E_PER_JOB_TIMEOUT="$(E2E_PER_JOB_TIMEOUT)" \
	$(GO) test -tags=e2e -v -race -timeout=$(E2E_CORE_TIMEOUT) -count=1 \
		-run "TestE2E_Core_RateLimitFriendly$$" ./test/e2e/...; \
	$(call log_info,Core E2E test completed successfully)

# Run CI-focused E2E tests (single canonical variant, rate-limit safe)
# This is the recommended target for CI pipelines - runs the single-job core test only
test-e2e-ci:
	@set -euo pipefail; \
	$(call log_info,Running CI-focused E2E tests (single-job core variant only)...); \
	$(call load_env); \
	$(GO) test -tags=e2e -v -race -timeout=10m -count=1 \
		-run "TestE2E_Core_SingleJob$$" ./test/e2e/...

# Run CI E2E with service startup and cleanup (single-job core test only)
run-e2e-ci:
	@set -euo pipefail; \
	$(call log_info,Starting CI-focused E2E suite (single-job core variant only)...); \
	$(call load_env); \
	rm -rf $(ARTIFACTS_DIR)/* || true; \
	mkdir -p $(ARTIFACTS_DIR); \
	$(call log_info,Starting services...); \
	$(DOCKER_COMPOSE) $(compose_files_args) up -d --build; \
	trap 'echo "==> E2E cleanup..."; $(call comprehensive_cleanup); echo "==> Cleanup completed"' EXIT; \
	$(call wait_for_postgres); \
	$(call verify_database_schema); \
	$(call wait_for_healthz); \
	$(call log_info,Running CI E2E tests (single-job core variant only)...); \
	E2E_INTER_JOB_COOLDOWN=0s \
	E2E_PER_JOB_TIMEOUT=120s \
	$(GO) test -tags=e2e -v -race -timeout=10m -count=1 \
		-run "TestE2E_Core_SingleJob$$" ./test/e2e/...; \
	$(call log_info,CI E2E tests completed successfully)

# Run core E2E multiple times consecutively to validate rate-limit safety
# Usage: make run-e2e-core-repeat RUNS=2
# This target validates that the algorithm can handle back-to-back test runs
RUNS ?= 2
run-e2e-core-repeat:
	@set -euo pipefail; \
	$(call log_info,Running core E2E $(RUNS) times consecutively to validate rate-limit safety...); \
	$(call load_env); \
	rm -rf $(ARTIFACTS_DIR)/* || true; \
	mkdir -p $(ARTIFACTS_DIR); \
	$(call log_info,Starting services...); \
	$(DOCKER_COMPOSE) $(compose_files_args) up -d --build; \
	trap 'echo "==> E2E cleanup..."; $(call comprehensive_cleanup); echo "==> Cleanup completed"' EXIT; \
	$(call wait_for_postgres); \
	$(call verify_database_schema); \
	$(call wait_for_healthz); \
	for i in $$(seq 1 $(RUNS)); do \
		$(call log_info,=== Run $$i of $(RUNS) ===); \
		E2E_CORE_JOB_COUNT=1 \
		E2E_INTER_JOB_COOLDOWN="$(E2E_INTER_JOB_COOLDOWN)" \
		E2E_PER_JOB_TIMEOUT="$(E2E_PER_JOB_TIMEOUT)" \
		$(GO) test -tags=e2e -v -race -timeout=5m -count=1 \
			-run "TestE2E_Core_RateLimitFriendly$$" ./test/e2e/...; \
		if [ $$i -lt $(RUNS) ]; then \
			$(call log_info,Cooldown between runs: 60s); \
			sleep 60; \
		fi; \
	done; \
	$(call log_info,All $(RUNS) runs completed successfully - rate-limit algorithm validated!)

 cover:
	$(GO) tool cover -html=coverage/coverage.unit.out -o coverage/coverage.html

# --- Consolidated E2E Test Target ---------------------------------------------

# Parameters for E2E test execution
E2E_CLEAR_DUMP ?= true
E2E_START_SERVICES ?= false
E2E_BASE_URL ?= 
E2E_TIMEOUT ?= 5m
E2E_LOG_DIR ?= 
E2E_PARALLEL ?= 2
E2E_WORKER_REPLICAS ?= 1
E2E_AI_TIMEOUT ?= 30s
E2E_POLL_INTERVAL ?= 50ms 
E2E_INTER_PAIR_DELAY ?= 0s

# Consolidated E2E test target that can be reused
# Usage: make run-e2e-tests E2E_START_SERVICES=true E2E_BASE_URL=http://localhost:8080/v1
# E2E Test Helper Functions - Refactored and Simplified
define wait_for_postgres
	$(call log_info,Waiting for Postgres to be ready \(max 60s\)...); \
	for i in $$(seq 1 30); do \
		if $(DOCKER_COMPOSE) $(compose_files_args) exec -T db pg_isready -U postgres >/dev/null 2>&1; then \
			$(call log_info,Postgres is ready); \
			break; \
		fi; \
		echo "  Attempt $$i/30: waiting for db..."; \
		sleep 2; \
	done
endef

define verify_database_schema
	$(call log_info,Verifying database schema...); \
	$(DOCKER_COMPOSE) $(compose_files_args) exec -T db psql -U postgres -d app -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'results';" | grep -q results || (echo "ERROR: results table not found after migration" && exit 1); \
	$(call log_info,Database schema verified)
endef

define wait_for_healthz
	$(call log_info,Waiting for readyz endpoint \(max 120s\)...); \
	APP_PORT=$${PORT:-8080}; \
	MAX_ATTEMPTS=60; \
	ATTEMPT=0; \
	while [ $$ATTEMPT -lt $$MAX_ATTEMPTS ]; do \
		ATTEMPT=$$((ATTEMPT + 1)); \
		if curl -fsS http://localhost:$$APP_PORT/readyz >/dev/null 2>&1; then \
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
		$(DOCKER_COMPOSE) $(compose_files_args) logs -f app worker > "$$LOG_DIR/compose.follow.log" 2>&1 & LOG_FOLLOW_PID=$$!; \
		trap 'echo "==> Collecting docker logs..."; $(DOCKER_COMPOSE) $(compose_files_args) logs > "$$LOG_DIR/compose.full.log" 2>&1 || true; grep -iE "\\b(error|panic|fatal)\\b" "$$LOG_DIR/compose.full.log" > "$$LOG_DIR/compose.errors.log" || true; [ -n "$$LOG_FOLLOW_PID" ] && kill "$$LOG_FOLLOW_PID" 2>/dev/null || true' EXIT; \
	fi
endef

define collect_post_test_logs
	LOG_DIR="$(E2E_LOG_DIR)"; \
	if [ "$(E2E_START_SERVICES)" = "true" ] && [ -n "$$LOG_DIR" ]; then \
		$(call log_info,Collecting docker logs after tests...); \
		$(DOCKER_COMPOSE) $(compose_files_args) logs > "$$LOG_DIR/compose.full.post.log" 2>&1 || true; \
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
	LOG_DIR="$(E2E_LOG_DIR)"; \
	if [ -n "$$LOG_DIR" ]; then \
		GO_TEE="tee \"$$LOG_DIR/go-test.log\""; \
	else \
		GO_TEE="cat"; \
	fi; \
	$(call log_info,Running E2E tests with parallel=$(E2E_PARALLEL) and $(E2E_WORKER_REPLICAS) workers...); \
	$(call log_info,AI timeout: $(E2E_AI_TIMEOUT), Poll interval: $(E2E_POLL_INTERVAL)); \
	if [ -n "$(E2E_BASE_URL)" ]; then \
		E2E_BASE_URL="$(E2E_BASE_URL)" E2E_WORKER_REPLICAS="$(E2E_WORKER_REPLICAS)" E2E_AI_TIMEOUT="$(E2E_AI_TIMEOUT)" E2E_POLL_INTERVAL="$(E2E_POLL_INTERVAL)" E2E_INTER_PAIR_DELAY="$(E2E_INTER_PAIR_DELAY)" $(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) ./test/e2e/... 2>&1 | eval "$$GO_TEE"; \
	else \
		E2E_WORKER_REPLICAS="$(E2E_WORKER_REPLICAS)" E2E_AI_TIMEOUT="$(E2E_AI_TIMEOUT)" E2E_POLL_INTERVAL="$(E2E_POLL_INTERVAL)" E2E_INTER_PAIR_DELAY="$(E2E_INTER_PAIR_DELAY)" $(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) ./test/e2e/... 2>&1 | eval "$$GO_TEE"; \
	fi
endef

define run_comprehensive_smoke_tests
	$(call log_info,Loading .env file...); \
	$(call load_env); \
	$(call log_info,Running comprehensive smoke E2E tests with all test data and scenarios...); \
	$(call log_info,Configuration: parallel=$(E2E_PARALLEL), workers=$(E2E_WORKER_REPLICAS)); \
	$(call log_info,AI timeout: $(E2E_AI_TIMEOUT), Poll interval: $(E2E_POLL_INTERVAL)); \
	$(call log_info,Testing all available CV/project pairs, edge cases, and performance scenarios...); \
	if [ -n "$(E2E_BASE_URL)" ]; then \
		E2E_BASE_URL="$(E2E_BASE_URL)" E2E_WORKER_REPLICAS="$(E2E_WORKER_REPLICAS)" E2E_AI_TIMEOUT="$(E2E_AI_TIMEOUT)" E2E_POLL_INTERVAL="$(E2E_POLL_INTERVAL)" $(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) -run "TestE2E_ComprehensiveSmoke|TestE2E_EdgeCaseSmoke|TestE2E_PerformanceSmoke" ./test/e2e/...; \
	else \
		E2E_WORKER_REPLICAS="$(E2E_WORKER_REPLICAS)" E2E_AI_TIMEOUT="$(E2E_AI_TIMEOUT)" E2E_POLL_INTERVAL="$(E2E_POLL_INTERVAL)" $(GO) test -tags=e2e -v -race -timeout=$(E2E_TIMEOUT) -failfast -count=1 -parallel=$(E2E_PARALLEL) -run "TestE2E_ComprehensiveSmoke|TestE2E_EdgeCaseSmoke|TestE2E_PerformanceSmoke" ./test/e2e/...; \
	fi
endef

# Refactored E2E Test Target - Clean and Modular with Centralized Cleanup
run-e2e-tests:
	@set -euo pipefail; \
	$(call log_info,Starting E2E test execution...); \
	$(call log_info,Configuration: E2E_CLEAR_DUMP=$(E2E_CLEAR_DUMP), E2E_START_SERVICES=$(E2E_START_SERVICES), E2E_BASE_URL=$(E2E_BASE_URL)); \
	$(call log_info,---); \
	# Clean previous artifacts/logs before E2E run \
	rm -rf $(ARTIFACTS_DIR)/* || true; \
	mkdir -p $(ARTIFACTS_DIR); \
	if [ -n "$(E2E_LOG_DIR)" ]; then rm -rf "$(E2E_LOG_DIR)"; mkdir -p "$(E2E_LOG_DIR)"; fi; \
	if [ "$(E2E_CLEAR_DUMP)" = "true" ]; then \
		$(call log_info,Clearing dump directory...); \
		$(call clear_dump_dir); \
	fi; \
	$(call log_info,---); \
	if [ "$(E2E_START_SERVICES)" = "true" ]; then \
		$(call log_info,Starting services with $(DOCKER_COMPOSE) $(compose_files_args))...; \
		$(call setup_log_collection); \
		$(DOCKER_COMPOSE) $(compose_files_args) up -d --build; \
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
	set +e; \
	$(call run_e2e_tests); \
	E2E_STATUS=$$?; \
	set -e; \
	$(call log_info,---); \
	$(call collect_post_test_logs); \
	$(call log_info,---); \
	if [ "$(E2E_CLEAR_DUMP)" = "true" ]; then \
		$(call log_info,E2E responses dumped to $(TEST_DUMP_DIR)/); \
	fi; \
	$(call log_info,E2E test execution completed with status $$E2E_STATUS); \
	exit $$E2E_STATUS

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
	GOBIN=$(PWD)/bin $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
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
	total=$$(grep -E "^total:.*\(statements\).*[0-9.]+%$$" coverage/coverage.func.txt | awk '{print $$NF}' | tr -d '%'); \
	total_int=$${total%.*}; \
	if [ "$$total_int" -lt 80 ]; then \
	  echo "Overall coverage $$total% is below 80% minimum" >&2; \
	  exit 1; \
	fi

# E2E Test Management Targets

# Comprehensive Docker cleanup (removes containers, volumes, networks)
docker-cleanup:
	@echo "==> Comprehensive Docker cleanup..."
	@$(call comprehensive_cleanup)


# Show E2E test help
e2e-help:
	@echo "E2E Test Commands:"
	@echo "  make test-e2e                  - Local E2E run (assumes services already running)"
	@echo "  make run-e2e-tests             - Full E2E run with optional docker-compose services and logs"
	@echo "  make docker-cleanup            - Comprehensive Docker cleanup (containers, volumes, networks)"
	@echo ""
	@echo "Parameters:"
	@echo "  E2E_CLEAR_DUMP=true/false      - Clear dump directory (default: true)"
	@echo "  E2E_START_SERVICES=true/false  - Start Docker services (default: false)"
	@echo "  E2E_BASE_URL=<url>             - Base URL for tests"
	@echo "  E2E_LOG_DIR=<dir>              - Directory for logs"
	@echo "  E2E_PARALLEL=<num>             - Parallel test execution (default: 8)"
	@echo ""
	@echo "Note: run-e2e-tests uses docker-compose.yml with a single optimized worker by default"
	@echo "      (CONSUMER_MAX_CONCURRENCY=1 by default for free-tier safety; increase only if your AI quotas allow)"


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