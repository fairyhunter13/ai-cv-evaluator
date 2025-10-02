# CI/CD and GitHub Actions

This document describes the continuous integration and deployment pipeline using GitHub Actions for the AI CV Evaluator service.

## 🎯 Overview

The CI/CD pipeline provides:
- **Automated testing** on pull requests and pushes
- **Security scanning** for vulnerabilities
- **Container building** and publishing
- **Automated deployment** to VPS
- **Quality gates** for code merges

## 🔄 CI Workflows

### Main CI Workflow (`.github/workflows/ci.yml`)

**Triggers**:
- Pull requests to main branch
- Pushes to main branch
- Manual dispatch

**Steps**:
1. **Setup Go** with caching
2. **Install dependencies** and tools
3. **Code quality checks** (lint, vet, vuln)
4. **Unit tests** with coverage
5. **E2E tests** with services
6. **Build Docker image**
7. **Upload artifacts**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  workflow_dispatch:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.22'
          cache: true
          
      - name: Install dependencies
        run: make deps
        
      - name: Format check
        run: make fmt
        
      - name: Lint
        run: make lint
        
      - name: Vet
        run: make vet
        
      - name: Vulnerability check
        run: make vuln
        
      - name: Unit tests
        run: make test
        env:
          CGO_ENABLED: 0
          
      - name: E2E tests
        run: make test-e2e
        env:
          POSTGRES_URL: postgres://user:pass@localhost:5432/test
          REDIS_URL: redis://localhost:6379
          QDRANT_URL: http://localhost:6333
          
      - name: Build Docker image
        run: make docker-build
        
      - name: Upload coverage
        uses: actions/upload-artifact@v3
        with:
          name: coverage-report
          path: coverage/
```

### Docker Publish Workflow (`.github/workflows/docker-publish.yml`)

**Triggers**:
- Tags matching `v*` pattern
- Manual dispatch

**Steps**:
1. **Build multi-architecture** image
2. **Push to GHCR** with tags
3. **Generate SBOM** (optional)
4. **Sign image** (optional)

```yaml
name: Docker Publish

on:
  push:
    tags: ['v*']
  workflow_dispatch:

jobs:
  publish:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
        
      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
          
      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
          tags: |
            type=ref,event=branch
            type=ref,event=pr
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=raw,value=latest,enable={{is_default_branch}}
            
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Deploy Workflow (`.github/workflows/deploy.yml`)

**Triggers**:
- Tags matching `v*` pattern
- Manual dispatch

**Steps**:
1. **Setup SSH** connection
2. **Login to GHCR** (if private)
3. **Pull latest image**
4. **Run migrations**
5. **Deploy with docker-compose**

```yaml
name: Deploy

on:
  push:
    tags: ['v*']
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup SSH
        uses: webfactory/ssh-agent@v0.7.0
        with:
          ssh-private-key: ${{ secrets.SSH_PRIVATE_KEY }}
          
      - name: Create remote directory
        run: ssh -o StrictHostKeyChecking=no ${{ secrets.SSH_USER }}@${{ secrets.SSH_HOST }} mkdir -p ${{ secrets.REMOTE_COMPOSE_PATH }}
        
      - name: Login to GHCR
        run: ssh -o StrictHostKeyChecking=no ${{ secrets.SSH_USER }}@${{ secrets.SSH_HOST }} docker login ghcr.io -u ${{ secrets.GHCR_USERNAME }} -p ${{ secrets.GHCR_TOKEN }}
        
      - name: Pull latest image
        run: ssh -o StrictHostKeyChecking=no ${{ secrets.SSH_USER }}@${{ secrets.SSH_HOST }} docker pull ${{ secrets.IMAGE_REF }}
        
      - name: Run migrations
        run: ssh -o StrictHostKeyChecking=no ${{ secrets.SSH_USER }}@${{ secrets.SSH_HOST }} docker run --rm --network host ${{ secrets.IMAGE_REF }} make migrate
        
      - name: Deploy application
        run: ssh -o StrictHostKeyChecking=no ${{ secrets.SSH_USER }}@${{ secrets.SSH_HOST }} docker compose -f ${{ secrets.REMOTE_COMPOSE_PATH }}/docker-compose.yml up -d
```

## 🔒 Secrets Management

### Required Secrets

#### SSH Deployment
- `SSH_HOST` - VPS hostname or IP
- `SSH_USER` - SSH username
- `SSH_PRIVATE_KEY` - SSH private key
- `REMOTE_COMPOSE_PATH` - Remote compose file path
- `IMAGE_REF` - Docker image reference

#### Container Registry (Optional)
- `GHCR_USERNAME` - GitHub Container Registry username
- `GHCR_TOKEN` - GitHub Container Registry token

#### SOPS Encryption (Optional)
- `SOPS_AGE_KEY` - Age key for SOPS decryption

### Secret Configuration
```bash
# Add secrets via GitHub CLI
gh secret set SSH_HOST --body "your-vps-host"
gh secret set SSH_USER --body "your-username"
gh secret set SSH_PRIVATE_KEY --body "$(cat ~/.ssh/id_rsa)"
gh secret set REMOTE_COMPOSE_PATH --body "/opt/ai-cv-evaluator"
gh secret set IMAGE_REF --body "ghcr.io/owner/repo:latest"
```

## 🧪 Testing in CI

### Unit Tests
```yaml
- name: Unit tests
  run: |
    go test -race -short -coverprofile=coverage.unit.out ./...
    go tool cover -html=coverage.unit.out -o coverage.unit.html
  env:
    CGO_ENABLED: 0
```

### E2E Tests
```yaml
- name: E2E tests
  run: |
    go test -tags=e2e -coverprofile=coverage.e2e.out ./test/e2e/...
    go tool cover -html=coverage.e2e.out -o coverage.e2e.html
  env:
    POSTGRES_URL: postgres://user:pass@localhost:5432/test
    REDIS_URL: redis://localhost:6379
    QDRANT_URL: http://localhost:6333
```

### Test Placement Enforcement
```yaml
- name: Enforce unit test placement
  shell: bash
  run: |
    set -euo pipefail
    # Find any *_test.go under top-level test/ except test/e2e/**
    if git ls-files -- ':!:test/e2e/**' 'test/**/_test.go' | grep -E '.+'; then
      echo "Misplaced unit tests detected under top-level test/. Move unit tests next to code (e.g., pkg/foo/foo_test.go)." >&2
      exit 1
    fi
```

## 🔍 Security Scanning

### Vulnerability Scanning
```yaml
- name: Vulnerability check
  run: |
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...
```

### Container Scanning
```yaml
- name: Container scan
  uses: aquasecurity/trivy-action@master
  with:
    image-ref: ${{ steps.meta.outputs.tags }}
    format: 'sarif'
    output: 'trivy-results.sarif'
```

### Naming Hygiene Check
```yaml
- name: Naming hygiene check
  shell: bash
  run: |
    set -euo pipefail
    if [ -f tools/naming_denylist.txt ]; then
      terms=$(grep -vE '^\s*(#|$)' tools/naming_denylist.txt || true)
      if [ -n "$terms" ]; then
        echo "$terms" | while IFS= read -r term; do
          if grep -RIn --exclude-dir=.git --binary-files=without-match -- "$term" .; then
            echo "Found disallowed term in repo content: $term" >&2
            exit 1
          fi
        done
      fi
    fi
```

## 📊 Caching and Performance

### Go Module Caching
```yaml
- name: Setup Go
  uses: actions/setup-go@v4
  with:
    go-version: '1.22'
    cache: true
```

### Docker Build Caching
```yaml
- name: Build and push
  uses: docker/build-push-action@v5
  with:
    cache-from: type=gha
    cache-to: type=gha,mode=max
```

### Warm Module Cache
```yaml
- name: Warm module cache
  run: go mod download
```

## 🚀 Deployment Strategy

### VPS Deployment
1. **SSH connection** to VPS
2. **Pull latest image** from GHCR
3. **Run database migrations**
4. **Deploy with docker-compose**
5. **Health check** verification

### Zero-Downtime Deployment
- **Reverse proxy** in front of application
- **Rolling updates** with multiple replicas
- **Health checks** before traffic routing
- **Rollback capability** to previous version

### Deployment Verification
```yaml
- name: Health check
  run: |
    curl -f http://${{ secrets.SSH_HOST }}/healthz || exit 1
    curl -f http://${{ secrets.SSH_HOST }}/readyz || exit 1
```

## 📈 Quality Gates

### Code Quality
- **Linting**: `golangci-lint` must pass
- **Formatting**: `gofmt` and `goimports` check
- **Vulnerabilities**: `govulncheck` must be clean
- **Security**: Container scanning must pass

### Testing
- **Unit tests**: Must pass with race detection
- **E2E tests**: Must pass with live services
- **Coverage**: Meet minimum thresholds
- **Performance**: Within acceptable limits

### Documentation
- **OpenAPI**: Must be valid and current
- **README**: Must be updated
- **Code comments**: Must be present for exports

## 🔄 Matrix and Concurrency

### Go Version Matrix
```yaml
strategy:
  matrix:
    go-version: ['1.22', '1.23']
    platform: [ubuntu-latest]
```

### Concurrency Control
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

## 📋 Artifacts and Reports

### Coverage Reports
```yaml
- name: Upload coverage
  uses: actions/upload-artifact@v3
  with:
    name: coverage-report
    path: coverage/
```

### Test Logs
```yaml
- name: Upload test logs
  uses: actions/upload-artifact@v3
  with:
    name: test-logs
    path: logs/
```

### Security Reports
```yaml
- name: Upload security report
  uses: actions/upload-artifact@v3
  with:
    name: security-report
    path: trivy-results.sarif
```

## ✅ Definition of Done (CI/CD)

### Implementation Requirements
- **CI runs automatically** on PRs and pushes
- **All quality gates** must pass
- **Docker image builds** successfully
- **Deployment works** end-to-end
- **Security scanning** integrated

### Performance Requirements
- **Build time** under 10 minutes
- **Test execution** under 5 minutes
- **Deployment time** under 2 minutes
- **Resource usage** optimized

### Reliability Requirements
- **No flaky tests** in CI
- **Consistent results** across runs
- **Rollback capability** for deployments
- **Monitoring** of CI/CD pipeline

This document serves as the comprehensive guide for CI/CD implementation, ensuring automated, reliable, and secure software delivery for the AI CV Evaluator service.
