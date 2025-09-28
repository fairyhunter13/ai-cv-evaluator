SHELL := /bin/bash

APP_NAME := ai-cv-evaluator
GO := go
GOFLAGS := -trimpath
GOTOOLCHAIN := auto
CGO_ENABLED ?= 0
SOPS_AGE_KEY_FILE ?= $(HOME)/.config/sops/age/keys.txt

.PHONY: all deps fmt lint vet vuln test test-int test-e2e cover run build docker-build docker-run migrate tools generate seed-rag \
	encrypt-env decrypt-env encrypt-env-production decrypt-env-production verify-project-sops encrypt-project decrypt-project ci-test ci-e2e

all: fmt lint vet test

 deps:
	$(GO) mod download

 fmt:
	gofmt -s -w .
	goimports -w . || true

 lint:
	golangci-lint run ./...

 vet:
	$(GO) vet ./...

# --- Secrets (SOPS) -----------------------------------------------------------

# Encrypt .env -> .env.sops.yaml using SOPS + age
encrypt-env:
	@[ -f .env ] || (echo "Error: .env not found. Create it first (you can copy from .env.sample)." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt .env > .env.sops.yaml
	@echo "Encrypted .env -> .env.sops.yaml"

# Decrypt .env.sops.yaml -> .env
decrypt-env:
	@[ -f .env.sops.yaml ] || (echo "Error: .env.sops.yaml not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d .env.sops.yaml > .env
	@echo "Decrypted .env.sops.yaml -> .env"

encrypt-env-production:
	@[ -f .env.production ] || (echo "Error: .env.production not found. Create it first (you can copy from .env.sample and adjust for prod)." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops --input-type dotenv --output-type yaml --encrypt .env.production > .env.production.sops.yaml
	@echo "Encrypted .env.production -> .env.production.sops.yaml"

decrypt-env-production:
	@[ -f .env.production.sops.yaml ] || (echo "Error: .env.production.sops.yaml not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d .env.production.sops.yaml > .env.production
	@echo "Decrypted .env.production.sops.yaml -> .env.production"

# Encrypt project.md -> project.md.sops (binary-safe)
encrypt-project:
	@[ -f project.md ] || (echo "Error: project.md not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -e project.md > project.md.sops
	@echo "Encrypted project.md -> project.md.sops"

# Decrypt project.md.sops -> project.md
decrypt-project:
	@[ -f project.md.sops ] || (echo "Error: project.md.sops not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d project.md.sops > project.md
	@echo "Decrypted project.md.sops -> project.md"

# Verify decrypted project equals source file (no diff)
verify-project-sops:
	@[ -f project.md.sops ] || (echo "Error: project.md.sops not found." && exit 1)
	@[ -f "$(SOPS_AGE_KEY_FILE)" ] || (echo "Error: SOPS age key not found at $(SOPS_AGE_KEY_FILE)" && exit 1)
	SOPS_AGE_KEY_FILE=$(SOPS_AGE_KEY_FILE) sops -d project.md.sops > project.md.dec
	@diff -u project.md project.md.dec && echo "OK: decrypted matches original" || (echo "Mismatch between project.md and decrypted project.md.sops" && rm -f project.md.dec && exit 1)
	@rm -f project.md.dec

 vuln:
	govulncheck ./...

 test:
	$(GO) test -race -short -coverprofile=coverage.unit.out ./...

 test-int:
	$(GO) test -tags=integration -coverprofile=coverage.int.out ./...

 test-e2e:
	$(GO) test -tags=e2e -v -timeout=5m ./test/e2e/...

 cover:
	go tool cover -html=coverage.unit.out -o coverage.html

 run:
	APP_ENV=dev $(GO) run ./cmd/server

 build:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server

 docker-build:
	docker build -t $(APP_NAME):local .

 docker-run:
	docker compose up -d --build

 migrate:
	$(GO) run github.com/pressly/goose/v3/cmd/goose@latest -dir ./deploy/migrations postgres "$$DB_URL" up

seed-rag:
	QDRANT_URL=$${QDRANT_URL:-http://localhost:6333} $(GO) run ./cmd/ragseed

 tools:
	GOBIN=$(PWD)/bin go install github.com/mgechev/revive@latest
	GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
	GOBIN=$(PWD)/bin go install golang.org/x/vuln/cmd/govulncheck@latest
	GOBIN=$(PWD)/bin go install gotest.tools/gotestsum@latest

 generate:
	$(GO) generate ./...

# --- CI convenience targets ---------------------------------------------------

ci-test:
	@set -euo pipefail; \
	$(GO) test -race -short -coverprofile=coverage.unit.out ./...; \
	go tool cover -func=coverage.unit.out | tee coverage.func.txt; \
	total=$$(grep -E "^total:\\s*\\(statements\\)\\s*[0-9.]+%$$" coverage.func.txt | awk '{print $$3}' | tr -d '%'); \
	total_int=$${total%.*}; \
	if [ "$$total_int" -lt 60 ]; then \
	  echo "Overall coverage $$total% is below 60% minimum" >&2; \
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

ci-e2e:
	@set -euo pipefail; \
	docker compose -f docker-compose.yml up -d db redis qdrant tika; \
	sleep 15; \
	DB_URL='postgres://postgres:postgres@localhost:5432/app?sslmode=disable' make migrate; \
	# Load .env if present so local E2E runs mirror dev env; values can still be overridden below \
	set -a; [ -f .env ] && . ./.env || true; set +a; \
	APP_ENV=dev \
	DB_URL='postgres://postgres:postgres@localhost:5432/app?sslmode=disable' \
	REDIS_URL='redis://localhost:6379' \
	QDRANT_URL='http://localhost:6333' \
	TIKA_URL='http://localhost:9998' \
	$(GO) run ./cmd/server & \
	SERVER_PID=$$!; \
	sleep 5; \
	timeout 300s $(GO) test -tags=e2e -v -timeout=5m ./test/e2e/... || TEST_EXIT_CODE=$$?; \
	kill $$SERVER_PID || true; \
	if [ "$$${TEST_EXIT_CODE:-0}" -ne 0 ]; then \
	  echo "E2E tests failed with exit code: $$TEST_EXIT_CODE"; \
	  exit $$TEST_EXIT_CODE; \
	fi
