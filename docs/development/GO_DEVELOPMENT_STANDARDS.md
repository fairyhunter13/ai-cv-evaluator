# Go Development Standards

This document outlines the comprehensive development standards for the AI CV Evaluator Go backend service, following Clean Architecture and DDD principles.

## 🎯 Core Principles

- **Clean Architecture**: Clear boundaries between domain, usecase, and adapter layers
- **Domain-Driven Design**: Business logic centered around domain entities
- **Idiomatic Go**: Following Go best practices and conventions
- **Production-Ready**: Testable, resilient, observable, and maintainable code

## 🏗️ Architecture & Project Layout

### Clean Architecture Boundaries

```
cmd/
├── server/main.go          # Bootstrap & DI
└── worker/main.go          # Worker bootstrap

internal/
├── domain/                 # Business types, constants, errors, pure logic
├── usecase/                # Application services orchestrating domain + ports
└── adapter/                # External concerns
    ├── http/               # Handlers, middleware, request/response DTO mapping
    ├── repo/               # DB repositories
    ├── queue/              # Background jobs
    ├── ai/                 # LLM + embeddings + RAG
    └── observability/     # Logging, tracing, metrics

pkg/                        # Shared utilities: error wrappers, validation, pagination, httpclient
api/                        # OpenAPI spec, schema fixtures
configs/                    # Config schema, .env.example, defaults
deploy/                     # Docker, compose, migrations, ops docs
test/                       # Testdata, e2e harness, mocks
```

### Dependency Direction
- **Domain** → **Usecase** → **Adapter**
- Handlers depend on usecases via interfaces
- Repos implement interfaces
- Prefer composition, small interfaces, clear boundaries
- No circular dependencies

## 💻 Coding Standards

### Function Design
- **Short, focused functions** with single responsibility
- **Clear naming** that describes intent
- **Avoid globals**; inject dependencies with constructors
- **Propagate context.Context** to all boundaries
- **Set timeouts** for IO calls

### Error Handling
- **Wrap errors with context** using `fmt.Errorf("op=name: %w", err)`
- **Sentinel error variables** in domain layer
- **Structured error taxonomy** across layers
- **Never leak internal traces** to clients

### Concurrency
- **Guard shared state** with proper synchronization
- **Cancel goroutines** when context is cancelled
- **Avoid goroutine leaks**
- **Use worker pools** when appropriate
- **Respect cancellation** from queue system

## 🔧 Go Version and Modules

- **Go 1.22+** required via `go.mod`
- **Minimal dependencies** and pinned versions
- **Module-aware** development

## 🛠️ Tooling

### Linting and Formatting
- **golangci-lint** with comprehensive linter set
- **gofmt** + **goimports** for formatting
- **govulncheck** for vulnerability scanning
- **gosec** for security analysis

### Code Generation
- **gomock** + **mockgen** for mocks
- **go:generate** directives near interfaces
- **gotestsum** for better CI output

### Makefile Targets
```bash
make deps          # Install dev tools locally
make fmt           # Format and import fix
make lint         # Run golangci-lint with config
make vet           # Run go vet ./...
make vuln          # Run govulncheck ./...
make test          # Unit tests
make test-int      # Integration tests
make test-e2e      # End-to-end API tests
make cover         # Coverage report (HTML artifact)
make run           # Run server using .env
make docker-build  # Container workflow
make docker-run    # Run with Docker
```

## 🧪 Testing Standards

### Test Placement (Strict)
- **Unit tests MUST be co-located** next to the code under test
- **Example**: `internal/usecase/service.go` → `internal/usecase/service_test.go`
- **Top-level `test/` tree** reserved for E2E suites (`test/e2e/`) and shared fixtures only
- **No unit tests** under top-level `test/` except `test/e2e/`

### Test Execution
```bash
# Unit tests (fast, race, coverage)
go test -v -race -timeout=60s -failfast -parallel=4 ./...

# E2E tests (with build tags)
go test -tags=e2e -v -race -failfast -count=1 -timeout=90s -parallel=4 ./test/e2e/...

# Coverage
go test -v -race -timeout=60s -failfast -parallel=4 -cover ./...
```

### Test Quality Standards
- **Coverage target**: ≥80% for core domain/usecase packages, ≥60% overall minimum
- **Race detection**: Always use `-race` flag
- **Deterministic tests**: Seed `math/rand` explicitly
- **Mock external boundaries**: LLM client, embeddings, Qdrant, Redis queue, DB repo
- **Table-driven tests** with `t.Run(tc.name, func(t *testing.T) { ... })`

## 🔒 Security Standards

### Input Validation
- **Sanitize all inputs** and validate file types
- **Enforce size limits** (10MB default for uploads)
- **Strip control characters** from extracted text
- **Use allowlists** over denylists

### Secrets Management
- **Never commit plaintext secrets**
- **Use SOPS** for encrypted secrets files
- **Environment variables** for configuration
- **Rotate keys regularly**

### HTTP Security Headers
```go
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
Content-Security-Policy: default-src 'none'
Referrer-Policy: no-referrer
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
```

## 📊 Observability

### Logging
- **Structured JSON logs** using `log/slog`
- **Include correlation IDs**: `request_id`, `trace_id`, `span_id`
- **No sensitive data** in logs
- **Appropriate log levels**

### Metrics (Prometheus)
- **HTTP request metrics**: latency, counters by route/status
- **Queue metrics**: queued, processing, completed, failed
- **AI call metrics**: by provider and outcome
- **Resource metrics**: CPU, memory, Go runtime

### Tracing (OpenTelemetry)
- **HTTP middleware** with tracing
- **Instrument all boundaries**: DB, Redis, Qdrant, outbound HTTP
- **W3C Trace Context** propagation
- **Export via OTLP** to collector

## 🚀 Performance Guidelines

### Code Performance
- **Avoid unnecessary allocations** in hot paths
- **Profile before tuning**
- **Use streaming** for large file reads
- **Cache stable derived data** with TTL

### Concurrency
- **Bounded worker pools**
- **Expose gauges** for in-flight jobs
- **Backpressure** via queue rate limiting
- **Respect cancellation** from queue system

## 🔄 Graceful Shutdown

### Shutdown Sequence
1. **Listen for SIGINT/SIGTERM**
2. **Stop accepting new work**
3. **Drain HTTP** with server shutdown context (30s timeout)
4. **Stop workers** and wait for in-flight jobs
5. **Persist processing → failed** if exceeded

## 📝 Documentation Standards

### Code Documentation
- **GoDoc comments** for exported types and functions
- **Clear examples** in documentation
- **Keep documentation current** with code changes

### API Documentation
- **OpenAPI specification** as source of truth
- **Validate handlers** against OpenAPI in E2E tests
- **Keep response shapes** in sync with spec

## ✅ Definition of Done

### Code Quality
- **Builds with Go 1.22+**
- **`go vet` and `golangci-lint` clean**
- **`govulncheck` clean**
- **Unit + E2E tests** cover core flows
- **OpenAPI describes** endpoints and models

### Testing
- **All tests pass** with race detection
- **Coverage meets** minimum requirements
- **E2E tests** validate live providers
- **No flaky tests** or race conditions

### Documentation
- **Code comments** for exported APIs
- **README updated** with new features
- **OpenAPI spec** current and valid
- **Architecture docs** reflect current design

## 🎯 Best Practices Summary

1. **Follow Clean Architecture** principles strictly
2. **Write comprehensive tests** with proper coverage
3. **Handle errors gracefully** with proper context
4. **Use structured logging** for observability
5. **Implement proper security** measures
6. **Document everything** clearly and concisely
7. **Keep dependencies minimal** and up-to-date
8. **Profile before optimizing** performance
9. **Use context for cancellation** and timeouts
10. **Maintain clear separation** of concerns

This document serves as the definitive guide for Go development in the AI CV Evaluator project, ensuring consistent, high-quality, and maintainable code across the entire codebase.
