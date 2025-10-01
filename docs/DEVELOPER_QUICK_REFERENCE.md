# Developer Quick Reference

## 🚀 **Quick Commands**

### **Development Setup**

#### Development Environment
```bash
# Start complete development environment (backend + frontend)
make dev-full

# Or start frontend separately
make frontend-dev
```

#### Backend Services
```bash
# Start backend services only (migrations run automatically)
docker compose up -d --build

# Run migrations manually (if needed)
make migrate

# Run tests
make ci-e2e

# View logs
docker compose logs worker
```

### **Testing**
```bash
# Unit tests
make test

# E2E tests (full stack)
make ci-e2e

# Linting
make lint

# Format code
make fmt
```

### **Development**

#### Frontend Development
```bash
# Install frontend dependencies
make frontend-install

# Start frontend dev server with HMR
make frontend-dev

# Build frontend for production
make frontend-build

# Clean frontend build artifacts
make frontend-clean
```

#### Backend Development
```bash
# Run server locally
make run

# Build Docker images
make docker-build

# Clean up
docker compose down -v
```

## 🏗️ **Current Architecture**

### **Split Architecture**
- **Frontend**: Vue 3 + Vite (HMR-enabled dev server)
- **Server**: 1 container (API-only HTTP requests)
- **Workers**: 8 containers × 30 concurrency = 240 workers
- **Queue**: Redpanda (Kafka-compatible)
- **Database**: PostgreSQL
- **Vector DB**: Qdrant
- **Text Extraction**: Apache Tika

### **Key Directories**
```
cmd/
├── server/          # HTTP server
└── worker/          # Background workers

internal/
├── domain/          # Business entities
├── usecase/         # Business logic
└── adapter/         # External integrations

admin-frontend/      # Vue 3 + Vite frontend
├── src/
│   ├── views/       # Page components
│   └── stores/      # Pinia state management
├── public/          # Static assets
└── package.json     # Frontend dependencies

docs/                # Documentation
test/e2e/            # E2E tests
```

## 📊 **Performance Metrics**

- **Job Processing**: 6-10 seconds average
- **Throughput**: 240 concurrent workers
- **Queue Priority**: default (10), critical (6), low (1)
- **Retry Logic**: Exponential backoff
- **Graceful Shutdown**: Asynq built-in handling

## 🔧 **Configuration**

### **Environment Variables**
```bash
# Core
APP_ENV=dev
DB_URL=postgres://postgres:postgres@db:5432/app?sslmode=disable
KAFKA_BROKERS=redpanda:9092

# AI Providers
OPENROUTER_API_KEY=your_key
OPENAI_API_KEY=your_key

# Vector DB
QDRANT_URL=http://qdrant:6333

# Observability
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
```

### **Docker Compose Services**
- `migrate`: Database migration container (runs once, then exits)
- `app`: Server container (API-only)
- `frontend`: Vue 3 frontend (development)
- `worker`: 8 worker replicas
- `db`: PostgreSQL
- `redpanda`: Queue backend
- `qdrant`: Vector database
- `tika`: Text extraction
- `prometheus`: Metrics
- `grafana`: Dashboards
- `jaeger`: Tracing

## 🐛 **Debugging**

### **Check Service Status**
```bash
docker compose ps
docker compose logs app
docker compose logs frontend
docker compose logs worker
```

### **Check Queue Status**
```bash
# Check Redpanda topics and consumer groups
docker exec ai-cv-evaluator-redpanda-1 rpk topic list
docker exec ai-cv-evaluator-redpanda-1 rpk group list
```

### **Check Database**
```bash
docker exec ai-cv-evaluator-db-1 psql -U postgres -d app -c "SELECT id, status FROM jobs ORDER BY created_at DESC LIMIT 5;"
```

### **Health Checks**
```bash
# Backend API
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/metrics

# Frontend (if running)
curl http://localhost:3001
```

## 📈 **Monitoring**

### **Grafana Dashboards**
- HTTP Metrics: http://localhost:3000
- Job Queue Metrics: http://localhost:3000
- AI Metrics: http://localhost:3000

### **Jaeger Tracing**
- Trace UI: http://localhost:16686

### **Prometheus Metrics**
- Metrics: http://localhost:9090

## 🔄 **Common Workflows**

### **Adding New Features**
1. Update domain entities
2. Add usecase logic
3. Create adapters
4. Add tests
5. Update documentation

### **Deploying Changes**
1. Update Docker images
2. Run migrations
3. Deploy with rolling update
4. Verify health checks

### **Troubleshooting**
1. Check service logs
2. Verify queue status
3. Check database state
4. Review metrics
5. Check health endpoints

## 📝 **Code Standards**

### **Architecture**
- Clean Architecture principles
- Domain-driven design
- Ports and adapters pattern

### **Testing**
- Unit tests for business logic
- E2E tests for full workflows

### **Documentation**
- Update README for user changes
- Update ARCHITECTURE.md for design changes
- Add ADRs for architectural decisions

## 🚨 **Common Issues**

### **Workers Not Processing**
- Check Redpanda connection
- Verify worker registration
- Check queue configuration

### **Jobs Stuck in Queued**
- Increase worker replicas
- Check AI provider keys
- Verify database connection

### **Performance Issues**
- Monitor worker metrics
- Check queue depth
- Verify resource limits

## 📚 **Documentation Links**

- [Complete Documentation](README.md)
- [Architecture Details](architecture/ARCHITECTURE.md)
- [Production Setup](production-split-architecture.md)
- [Contributing Guide](contributing/CONTRIBUTING.md)
- [Security Policy](security/SECURITY.md)

---

**Last Updated**: September 2024  
**Version**: 2.0 (Split Architecture)
