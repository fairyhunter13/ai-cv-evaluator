---
description: Run Golang linting
auto_execution_mode: 3
---

Steps to run Golang formatting and linting consistently in this repo.

1. Save any Go file to trigger the workflow automatically.
2. From the workspace root run `make fmt lint vet`.
   - `make fmt` reformats the repo with `gofmt`/`goimports` and installs `goimports` on demand.
   - `make lint` runs `golangci-lint`; fix every finding before continuing.
   - `make vet` performs `go vet` checks.
3. If any command exits non-zero, resolve the reported issues and rerun `make fmt lint vet` until all commands succeed.

// turbo
4. One-shot run (combined):
   - make fmt lint vet