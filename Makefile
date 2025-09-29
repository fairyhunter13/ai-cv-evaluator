SHELL := /bin/bash

APP_NAME := ai-cv-evaluator
GO := go
GOFLAGS := -trimpath
GOTOOLCHAIN := auto
CGO_ENABLED ?= 0
SOPS_AGE_KEY_FILE ?= $(HOME)/.config/sops/age/keys.txt

.PHONY: all deps fmt lint vet vuln test test-e2e cover run build docker-build docker-build-ci docker-run migrate tools generate seed-rag \
	encrypt-env decrypt-env encrypt-env-production decrypt-env-production verify-project-sops encrypt-project decrypt-project \
	encrypt-rfcs decrypt-rfcs build-admin-css \
	ci-test ci-e2e openapi-validate build-matrix verify-test-placement vendor-redismock gosec-sarif license-scan

all: fmt lint vet test

 deps:
	$(GO) mod download

 fmt:
	gofmt -s -w .
	goimports -w . || true

 lint:
	@which golangci-lint >/dev/null 2>&1 || (echo "Installing golangci-lint..." && GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1)
	golangci-lint run ./...

 vet:
	$(GO) vet ./...

# --- Secrets (SOPS) -----------------------------------------------------------

# Encrypt .env -> secrets/env.sops.yaml (YAML) using SOPS + age
encrypt-env:
	@[ -f .env ] || (echo "Error: .env not found. Create it first (you can copy from .env.sample)." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	@mkdir -p secrets
	cp .env secrets/env.sops.yaml
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt --in-place secrets/env.sops.yaml
	@echo "Encrypted .env -> secrets/env.sops.yaml"

# Decrypt secrets/env.sops.yaml -> .env
decrypt-env:
	@[ -f secrets/env.sops.yaml ] || (echo "Error: secrets/env.sops.yaml not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type yaml --output-type dotenv secrets/env.sops.yaml > .env
	@echo "Decrypted secrets/env.sops.yaml -> .env"

# Encrypt .env.production -> secrets/env.production.sops.yaml (YAML) using SOPS + age
encrypt-env-production:
	@[ -f .env.production ] || (echo "Error: .env.production not found. Create it first (you can copy from .env.sample and adjust for prod)." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	@mkdir -p secrets
	cp .env.production secrets/env.production.sops.yaml
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt --in-place secrets/env.production.sops.yaml
	@echo "Encrypted .env.production -> secrets/env.production.sops.yaml"

# Decrypt secrets/env.production.sops.yaml -> .env.production
decrypt-env-production:
	@[ -f secrets/env.production.sops.yaml ] || (echo "Error: secrets/env.production.sops.yaml not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type yaml --output-type dotenv secrets/env.production.sops.yaml > .env.production
	@echo "Decrypted secrets/env.production.sops.yaml -> .env.production"

# Encrypt docs/project.md -> secrets/project.md.enc (Binary)
encrypt-project:
	@[ -f docs/project.md ] || (echo "Error: docs/project.md not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	@mkdir -p secrets
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --encrypt --input-type binary --output-type binary docs/project.md > secrets/project.md.enc
	@echo "Encrypted docs/project.md -> secrets/project.md.enc"

# Decrypt secrets/project.md.enc -> docs/project.md
decrypt-project:
	@[ -f secrets/project.md.enc ] || (echo "Error: secrets/project.md.enc not found. Run 'make encrypt-project' first." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	@mkdir -p docs
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --decrypt --input-type binary --output-type binary secrets/project.md.enc > docs/project.md
	@echo "Decrypted secrets/project.md.enc -> docs/project.md"

# Verify decrypted project equals source file (no diff)
# Use secrets/project.md.sops as the canonical encrypted artifact for project.md
verify-project-sops:
	@[ -f secrets/project.md.sops ] || (echo "Error: secrets/project.md.sops not found." && exit 1)
	@mkdir -p docs
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d secrets/project.md.sops > docs/project.dec.md
	@diff -u docs/project.md docs/project.dec.md && echo "OK: decrypted matches original" || (echo "Mismatch between docs/project.md and decrypted secrets/project.md.sops" && rm -f docs/project.dec.md && exit 1)
	@rm -f docs/project.dec.md

# Encrypt all RFC markdowns under docs/rfc/** -> secrets/rfc/** (binary .sops)
encrypt-rfcs:
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
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

# Decrypt all secrets/rfc/**.sops -> docs/rfc/** (binary)
decrypt-rfcs:
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
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

 vuln:
	govulncheck ./...

 test:
	@pkgs=$$(go list ./... | grep -v "/cmd/"); \
	$(GO) test -race -short -coverprofile=coverage.unit.out $$pkgs

 test-e2e:
	E2E_FAST=1 $(GO) test -tags=e2e -v -timeout=10s ./test/e2e/...

 cover:
	go tool cover -html=coverage.unit.out -o coverage.html

 run:
	APP_ENV=dev $(GO) run ./cmd/server

 build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server

 docker-build:
	docker build -t $(APP_NAME):local .

docker-build-ci:
	@[ -n "$(TAG)" ] || (echo "Usage: make docker-build-ci TAG=<tag>" && exit 1)
	docker build -t $(APP_NAME):$(TAG) .

 docker-run:
	docker compose up -d --build

 migrate:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@latest -dir ./deploy/migrations postgres "$$DB_URL" up

 tools:
	GOBIN=$(PWD)/bin go install github.com/mgechev/revive@latest
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
	GOBIN=$(PWD)/bin go install golang.org/x/vuln/cmd/govulncheck@latest
	GOBIN=$(PWD)/bin go install gotest.tools/gotestsum@latest

# Build Tailwind CSS for Admin UI
build-admin-css:
	@set -euo pipefail; \
	if ! command -v npx >/dev/null 2>&1; then echo "Error: npx not found. Install Node.js (>=18)" >&2; exit 1; fi; \
	mkdir -p internal/adapter/httpserver/static; \
	# Try @tailwindcss/cli (v4+), then legacy tailwindcss (v3) as last resort
	(npx --yes @tailwindcss/cli@latest -i assets/tailwind.css -o internal/adapter/httpserver/static/admin.css --minify) \
	|| (npx --yes -p @tailwindcss/cli@latest tailwindcss -i assets/tailwind.css -o internal/adapter/httpserver/static/admin.css --minify) \
	|| (npm exec --yes @tailwindcss/cli@latest -- -i assets/tailwind.css -o internal/adapter/httpserver/static/admin.css --minify) \
	|| (npx --yes tailwindcss@latest -i assets/tailwind.css -o internal/adapter/httpserver/static/admin.css --minify)

# Security scan: gosec with SARIF output
gosec-sarif:
	@which gosec >/dev/null 2>&1 || go install github.com/securego/gosec/v2/cmd/gosec@latest
	$(GO) env GOPATH >/dev/null 2>&1 || true
	$$(go env GOPATH)/bin/gosec -fmt sarif -out gosec-results.sarif ./... || true

# License scanning via FOSSA CLI
license-scan:
	@which fossa >/dev/null 2>&1 || go install github.com/fossa-contrib/fossa-cli@latest
	$(GO) mod download
	$$(go env GOPATH)/bin/fossa analyze

 generate:
	$(GO) generate ./...

openapi-validate:
	$(GO) run github.com/getkin/kin-openapi/cmd/validate@latest api/openapi.yaml

build-matrix:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/server-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/server-linux-arm64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="-w -s" -o dist/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="-w -s" -o dist/server-darwin-arm64 ./cmd/server

verify-test-placement:
	@set -euo pipefail; \
	if git ls-files -- '*.go' | grep -E '^test/.*_test\.go$$' | grep -vE '^test/e2e/' ; then \
	  echo "Error: unit tests must be colocated next to code. Move tests out of top-level test/ (allowed only under test/e2e/)." >&2; \
	  exit 1; \
	fi

# --- CI convenience targets ---------------------------------------------------


ci-test:
	@set -euo pipefail; \
	pkgs=$$(go list ./... | grep -v "/cmd/"); \
	$(GO) test -race -short -coverprofile=coverage.unit.out $$pkgs; \
	go tool cover -func=coverage.unit.out | tee coverage.func.txt; \
	total=$$(grep -E "^total:\\s*\\(statements\\)\\s*[0-9.]+%$$" coverage.func.txt | awk '{print $$3}' | tr -d '%'); \
	total_int=$${total%.*}; \
	if [ "$$total_int" -lt 80 ]; then \
	  echo "Overall coverage $$total% is below 80% minimum" >&2; \
	  exit 1; \
	fi; \
	$(GO) test -race -short -coverprofile=coverage.core.out ./internal/domain ./internal/usecase; \
	go tool cover -func=coverage.core.out | tee coverage.core.func.txt; \
	total=$$(grep -E "^total:\\s*\\(statements\\)\\s*[0-9.]+%$$" coverage.core.func.txt | awk '{print $$3}' | tr -d '%'); \
	total_int=$${total%.*}; \
	if [ "$$total_int" -lt 80 ]; then \
	  echo "Core coverage $$total% is below 80% target" >&2; \
	  exit 1; \
	fi

vendor-redismock:
	@set -euo pipefail; \
	$(GO) get github.com/go-redis/redismock/v9; \
	$(GO) mod vendor; \
	echo "Vendored github.com/go-redis/redismock/v9"

ci-e2e:
	@set -euo pipefail; \
	# Start all services including app so tests hit containerized server \
	docker compose -f docker-compose.yml up -d --build; \
	# Wait a bit for healthchecks to pass \
	sleep 20; \
	# Run migrations against local forwarded DB port \
	DB_URL='postgres://postgres:postgres@localhost:5432/app?sslmode=disable' make migrate; \
	# Wait for app readiness endpoint to respond \
	echo 'Waiting for app readiness...'; \
	READY=0; \
	for i in $$(seq 1 30); do \
	  if curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; then \
	    READY=1; echo 'App is ready.'; break; \
	  fi; \
	  echo "Attempt $$i: app not ready yet..."; \
	  sleep 3; \
	done; \
	if [ "$$READY" -ne 1 ]; then \
	  echo 'App did not become ready in time'; \
	  docker compose -f docker-compose.yml logs --no-color --tail=200 app || true; \
	  exit 1; \
	fi; \
	# Load .env for test process (ADMIN_*, OPENAI_*, OPENROUTER_*, etc.) \
	set -a; [ -f .env ] && . ./.env || true; set +a; \
	# Execute E2E tests with verbose output (Go's -timeout enforces runtime limit) \
	$(GO) test -tags=e2e -v -timeout=5m ./test/e2e/... || TEST_EXIT_CODE=$$?; \
	# Show recent logs to aid debugging \
	echo '--- docker compose ps ---'; docker compose -f docker-compose.yml ps; \
	echo '--- app logs (last 200 lines) ---'; docker compose -f docker-compose.yml logs --no-color --tail=200 app || true; \
	if [ "$$${TEST_EXIT_CODE:-0}" -ne 0 ]; then \
	  echo "E2E tests failed with exit code: $$TEST_EXIT_CODE"; \
	  exit $$TEST_EXIT_CODE; \
	fi
