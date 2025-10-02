# Docker and Local Development

This document describes the containerization strategy and local development setup for the AI CV Evaluator service.

## 🎯 Overview

The Docker setup provides:
- **Multi-stage builds** for optimized images
- **Local development** with hot reloading
- **Service orchestration** with docker-compose
- **Production-ready** containerization

## 🐳 Dockerfile Design

### Multi-Stage Build

#### Builder Stage
```dockerfile
FROM golang:1.22-bookworm AS builder

# Set build arguments
ARG CGO_ENABLED=0
ARG GOOS=linux
ARG GOARCH=amd64

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -ldflags="-s -w" -o main ./cmd/server

# Build worker
RUN go build -ldflags="-s -w" -o worker ./cmd/worker
```

#### Runtime Stage
```dockerfile
FROM gcr.io/distroless/base-debian12:latest

# Copy binaries
COPY --from=builder /app/main /app/main
COPY --from=builder /app/worker /app/worker

# Set working directory
WORKDIR /app

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/main", "health"]

# Run as non-root user
USER 65534:65534

# Start application
ENTRYPOINT ["/app/main"]
```

### Worker Dockerfile
```dockerfile
FROM gcr.io/distroless/base-debian12:latest

# Copy worker binary
COPY --from=builder /app/worker /app/worker

# Set working directory
WORKDIR /app

# Run as non-root user
USER 65534:65534

# Start worker
ENTRYPOINT ["/app/worker"]
```

## 🐙 Docker Compose Configuration

### Local Development

#### `docker-compose.yml`
```yaml
version: '3.8'

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "${PORT:-8080}:8080"
    environment:
      - APP_ENV=dev
      - PORT=8080
      - DB_URL=postgres://user:pass@db:5432/ai_cv_evaluator?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - QDRANT_URL=http://qdrant:6333
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
      qdrant:
        condition: service_healthy
    volumes:
      - ./logs:/app/logs
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  worker:
    build:
      context: .
      dockerfile: Dockerfile.worker
    environment:
      - APP_ENV=dev
      - DB_URL=postgres://user:pass@db:5432/ai_cv_evaluator?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - QDRANT_URL=http://qdrant:6333
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
      qdrant:
        condition: service_healthy
    volumes:
      - ./logs:/app/logs
    restart: unless-stopped

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=ai_cv_evaluator
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=pass
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./deploy/migrations:/docker-entrypoint-initdb.d
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U user -d ai_cv_evaluator"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5
    restart: unless-stopped

  qdrant:
    image: qdrant/qdrant:latest
    environment:
      - QDRANT__SERVICE__API_KEY=${QDRANT_API_KEY:-}
    volumes:
      - qdrant_data:/qdrant/storage
    ports:
      - "6333:6333"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6333/collections"]
      interval: 10s
      timeout: 3s
      retries: 10
    restart: unless-stopped

  tika:
    image: apache/tika:latest
    ports:
      - "9998:9998"
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  qdrant_data:
```

### Production Configuration

#### `docker-compose.prod.yml`
```yaml
version: '3.8'

services:
  app:
    image: ghcr.io/owner/ai-cv-evaluator:latest
    ports:
      - "8080:8080"
    environment:
      - APP_ENV=prod
      - PORT=8080
    env_file:
      - .env.prod
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
      qdrant:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3

  worker:
    image: ghcr.io/owner/ai-cv-evaluator:latest
    command: ["/app/worker"]
    environment:
      - APP_ENV=prod
    env_file:
      - .env.prod
    depends_on:
      db:
        condition: service_healthy
      redis:
        condition: service_healthy
      qdrant:
        condition: service_healthy
    restart: unless-stopped

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=ai_cv_evaluator
      - POSTGRES_USER=${DB_USER}
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${DB_USER} -d ai_cv_evaluator"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  qdrant:
    image: qdrant/qdrant:latest
    environment:
      - QDRANT__SERVICE__API_KEY=${QDRANT_API_KEY}
    volumes:
      - qdrant_data:/qdrant/storage
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6333/collections"]
      interval: 10s
      timeout: 3s
      retries: 10

volumes:
  postgres_data:
  redis_data:
  qdrant_data:
```

## 🔧 Development Setup

### Prerequisites
- **Docker** 20.10+
- **Docker Compose** 2.0+
- **Git** for version control

### Quick Start
```bash
# Clone repository
git clone https://github.com/owner/ai-cv-evaluator.git
cd ai-cv-evaluator

# Copy environment file
cp .env.example .env

# Start services
docker compose up -d

# Check status
docker compose ps

# View logs
docker compose logs -f app
```

### Environment Configuration

#### `.env.example`
```bash
# Application
APP_ENV=dev
PORT=8080

# Database
DB_URL=postgres://user:pass@localhost:5432/ai_cv_evaluator?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379

# Qdrant
QDRANT_URL=http://localhost:6333
QDRANT_API_KEY=

# AI Providers
OPENROUTER_API_KEY=
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=
OPENAI_BASE_URL=https://api.openai.com/v1
EMBEDDINGS_MODEL=text-embedding-3-small

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_SERVICE_NAME=ai-cv-evaluator
```

### Development Commands
```bash
# Start all services
docker compose up -d

# Start specific service
docker compose up -d app

# View logs
docker compose logs -f app
docker compose logs -f worker

# Execute commands in container
docker compose exec app /app/main --help
docker compose exec worker /app/worker --help

# Stop services
docker compose down

# Stop and remove volumes
docker compose down -v

# Rebuild images
docker compose build --no-cache

# Update services
docker compose pull
docker compose up -d
```

## 🚀 Production Deployment

### Image Building
```bash
# Build for multiple architectures
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/owner/ai-cv-evaluator:latest --push .

# Build for specific architecture
docker build --platform linux/amd64 -t ghcr.io/owner/ai-cv-evaluator:latest .
```

### Image Optimization
- **Multi-stage builds** to reduce image size
- **Distroless base** for security
- **Non-root user** for security
- **Minimal dependencies** in final image

### Security Considerations
- **Scan images** for vulnerabilities
- **Use specific tags** instead of latest
- **Regular updates** of base images
- **Secrets management** via environment variables

## 📊 Monitoring and Observability

### Health Checks
```yaml
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:8080/healthz"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Logging
```yaml
logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

### Metrics
- **Prometheus** scraping from `/metrics`
- **Health endpoints** for liveness/readiness
- **Resource monitoring** via Docker stats

## 🔄 Development Workflow

### Hot Reloading
```yaml
# For development with hot reloading
app:
  build:
    context: .
    dockerfile: Dockerfile.dev
  volumes:
    - .:/app
    - /app/vendor
  command: ["go", "run", "./cmd/server"]
```

### Database Migrations
```bash
# Run migrations
docker compose exec app /app/main migrate

# Create new migration
docker compose exec app /app/main migrate create add_new_table

# Rollback migration
docker compose exec app /app/main migrate rollback
```

### Testing
```bash
# Run unit tests
docker compose exec app go test ./...

# Run E2E tests
docker compose exec app go test -tags=e2e ./test/e2e/...

# Run with coverage
docker compose exec app go test -cover ./...
```

## 🛠️ Troubleshooting

### Common Issues

#### Port Conflicts
```bash
# Check port usage
netstat -tulpn | grep :8080

# Change port in docker-compose.yml
ports:
  - "8081:8080"
```

#### Database Connection Issues
```bash
# Check database status
docker compose exec db pg_isready -U user

# View database logs
docker compose logs db

# Connect to database
docker compose exec db psql -U user -d ai_cv_evaluator
```

#### Service Dependencies
```bash
# Check service health
docker compose ps

# View service logs
docker compose logs service-name

# Restart specific service
docker compose restart service-name
```

### Debugging
```bash
# Enter container shell
docker compose exec app sh

# View container processes
docker compose exec app ps aux

# Check environment variables
docker compose exec app env

# View container resources
docker stats
```

## 📋 Best Practices

### Development
- **Use .env files** for configuration
- **Mount source code** for hot reloading
- **Separate dev/prod** configurations
- **Use health checks** for dependencies

### Production
- **Use specific image tags**
- **Implement proper logging**
- **Configure resource limits**
- **Use secrets management**

### Security
- **Scan images** regularly
- **Use non-root users**
- **Limit container capabilities**
- **Implement network policies**

## ✅ Definition of Done (Docker)

### Implementation Requirements
- **Multi-stage builds** working
- **Docker Compose** starts all services
- **Health checks** functional
- **Environment configuration** working
- **Production deployment** tested

### Performance Requirements
- **Build time** under 5 minutes
- **Image size** optimized
- **Startup time** under 30 seconds
- **Resource usage** reasonable

### Security Requirements
- **Vulnerability scanning** clean
- **Non-root execution** verified
- **Secrets management** secure
- **Network isolation** implemented

This document serves as the comprehensive guide for Docker and local development, ensuring consistent, secure, and efficient containerized development and deployment.
