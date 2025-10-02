# Environment Variables Reference

This document provides a comprehensive reference for all environment variables used in the AI CV Evaluator project.

## Overview

The application uses environment variables for configuration across different environments (development, testing, production). All variables are loaded through the `internal/config` package using the `env` library.

## Core Application Variables

### Application Environment
```bash
# Application environment (dev, test, prod)
APP_ENV=dev                    # Default: dev
PORT=8080                      # Default: 8080
```

### Database Configuration
```bash
# PostgreSQL connection string
DB_URL=postgres://postgres:postgres@localhost:5432/app?sslmode=disable
```

**Production Example:**
```bash
DB_URL=postgres://user:password@db-host:5432/ai_cv_evaluator?sslmode=require
```

## Queue System (Redpanda/Kafka)

### Redpanda Configuration
```bash
# Kafka brokers (comma-separated)
KAFKA_BROKERS=localhost:19092,localhost:19093,localhost:19094
```

**Production Example:**
```bash
KAFKA_BROKERS=redpanda-1:9092,redpanda-2:9092,redpanda-3:9092
```

### Retry and DLQ Configuration
```bash
# Retry settings
RETRY_MAX_RETRIES=3                    # Maximum retry attempts
RETRY_INITIAL_DELAY=2s                # Initial retry delay
RETRY_MAX_DELAY=30s                    # Maximum retry delay
RETRY_MULTIPLIER=2.0                   # Exponential backoff multiplier
RETRY_JITTER=true                      # Enable jitter for backoff

# DLQ settings
DLQ_ENABLED=true                       # Enable DLQ functionality
DLQ_MAX_AGE=168h                       # DLQ job retention period (7 days)
DLQ_CLEANUP_INTERVAL=24h               # DLQ cleanup interval
```

### Queue Consumer Configuration
```bash
# Consumer settings
CONSUMER_MAX_CONCURRENCY=8            # Maximum concurrent consumers
```

## AI Provider Configuration

### OpenRouter (Chat Completions)
```bash
# OpenRouter API configuration
OPENROUTER_API_KEY=your_openrouter_api_key
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1    # Default
FREE_MODELS_REFRESH=1h                              # How often to refresh free models
```

### AI Backoff Configuration
```bash
# AI request backoff settings
AI_BACKOFF_MAX_ELAPSED_TIME=180s                    # Maximum backoff time
AI_BACKOFF_INITIAL_INTERVAL=2s                      # Initial backoff interval
AI_BACKOFF_MAX_INTERVAL=20s                         # Maximum backoff interval
AI_BACKOFF_MULTIPLIER=1.5                           # Backoff multiplier
```
```

### OpenAI (Embeddings)
```bash
# OpenAI API configuration
OPENAI_API_KEY=your_openai_api_key
OPENAI_BASE_URL=https://api.openai.com/v1          # Default
EMBEDDINGS_MODEL=text-embedding-3-small            # Default
```

### Free Models Configuration
```bash
# Free models refresh interval
FREE_MODELS_REFRESH=1h                             # Default: 1h
```

## Vector Database (Qdrant)

### Qdrant Configuration
```bash
# Qdrant connection
QDRANT_URL=http://localhost:6333                   # Default
QDRANT_API_KEY=your_qdrant_api_key                # Optional
```

**Production Example:**
```bash
QDRANT_URL=https://qdrant-cluster.example.com:6333
QDRANT_API_KEY=your_production_api_key
```

## Text Extraction (Apache Tika)

### Tika Configuration
```bash
# Apache Tika server URL
TIKA_URL=http://tika:9998                          # Default
```

**Production Example:**
```bash
TIKA_URL=https://tika-server.example.com:9998
```

## Observability Configuration

### OpenTelemetry
```bash
# OpenTelemetry configuration
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317    # Default: empty
OTEL_SERVICE_NAME=ai-cv-evaluator                 # Default
```

**Production Example:**
```bash
OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.example.com:4317
OTEL_SERVICE_NAME=ai-cv-evaluator-prod
```

## Security and Rate Limiting

### Rate Limiting
```bash
# Rate limiting configuration
RATE_LIMIT_PER_MIN=200                             # Default: 200
```

### CORS Configuration
```bash
# CORS allowed origins (comma-separated)
CORS_ALLOW_ORIGINS=http://localhost:3001,https://app.example.com
```

### Session Configuration
```bash
# Admin session configuration
ADMIN_USERNAME=admin                               # Default: admin
ADMIN_PASSWORD=changeme                            # Default: changeme
ADMIN_SESSION_SECRET=your-secret-key              # Required in production
```

## File Upload Configuration

### Upload Limits
```bash
# Maximum upload size in MB
MAX_UPLOAD_MB=10                                   # Default: 10
```

### File Type Validation
```bash
# Allowed MIME types (comma-separated)
ALLOWED_MIME_TYPES=text/plain,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document
```

## Data Retention and Cleanup

### Data Retention
```bash
# Data retention in days
DATA_RETENTION_DAYS=30                             # Default: 30
CLEANUP_INTERVAL=24h                               # Default: 24h
```

## Frontend Configuration

### Frontend Separation
```bash
# Enable API-only mode (separate frontend)
FRONTEND_SEPARATED=true                            # Default: false
```

### Frontend Development
```bash
# Frontend development server
NODE_ENV=development                               # Default: development
VITE_API_BASE_URL=http://localhost:8080           # Frontend API base URL
```

## AI Backoff Configuration

### Retry and Timeout Settings
```bash
# AI request backoff configuration
AI_BACKOFF_MAX_ELAPSED_TIME=90s                   # Default: 90s
AI_BACKOFF_INITIAL_INTERVAL=1s                    # Default: 1s
AI_BACKOFF_MAX_INTERVAL=10s                       # Default: 10s
AI_BACKOFF_MULTIPLIER=2.0                         # Default: 2.0
```

**Test Environment Overrides:**
- `APP_ENV=test` automatically uses faster timeouts (5s max elapsed time)

## Embedding Cache Configuration

### Cache Settings
```bash
# Embedding cache size
EMBED_CACHE_SIZE=2048                             # Default: 2048
```

## License Configuration

### Unidoc License (Optional)
```bash
# Unidoc license for PDF processing
UNIDOC_LICENSE_API_KEY=your_unidoc_license_key    # Optional
```

## Environment-Specific Configurations

### Development Environment
```bash
# .env for development
APP_ENV=dev
PORT=8080
DB_URL=postgres://postgres:postgres@localhost:5432/app?sslmode=disable
KAFKA_BROKERS=localhost:19092
QDRANT_URL=http://localhost:6333
TIKA_URL=http://tika:9998
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
RATE_LIMIT_PER_MIN=200
MAX_UPLOAD_MB=1
FRONTEND_SEPARATED=false
```

### Production Environment
```bash
# .env.production for production
APP_ENV=prod
PORT=8080
DB_URL=postgres://user:password@db-host:5432/ai_cv_evaluator?sslmode=require
KAFKA_BROKERS=redpanda-1:9092,redpanda-2:9092,redpanda-3:9092
QDRANT_URL=https://qdrant-cluster.example.com:6333
QDRANT_API_KEY=your_production_api_key
TIKA_URL=https://tika-server.example.com:9998
OTEL_EXPORTER_OTLP_ENDPOINT=https://otel-collector.example.com:4317
OTEL_SERVICE_NAME=ai-cv-evaluator-prod
RATE_LIMIT_PER_MIN=1000
MAX_UPLOAD_MB=50
CORS_ALLOW_ORIGINS=https://app.example.com
ADMIN_SESSION_SECRET=your-production-secret-key
FRONTEND_SEPARATED=true
```

### Test Environment
```bash
# .env.test for testing
APP_ENV=test
PORT=8080
DB_URL=postgres://postgres:postgres@localhost:5432/app_test?sslmode=disable
KAFKA_BROKERS=localhost:19092
QDRANT_URL=http://localhost:6333
TIKA_URL=http://tika:9998
# AI backoff automatically uses fast timeouts
```

## Docker Environment Variables

### Docker Compose Overrides
```yaml
# docker-compose.override.yml
services:
  app:
    environment:
      - APP_ENV=dev
      - MAX_UPLOAD_MB=1
      - PORT=8080
      - DB_URL=postgres://postgres:postgres@db:5432/app?sslmode=disable
      - KAFKA_BROKERS=redpanda:9092
      - QDRANT_URL=http://qdrant:6333
      - TIKA_URL=http://tika:9998
      - OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317
      - RATE_LIMIT_PER_MIN=200
```

## Validation and Requirements

### Required Variables (Production)
- `DB_URL` - Database connection string
- `KAFKA_BROKERS` - Queue system brokers
- `OPENROUTER_API_KEY` - AI chat completions
- `ADMIN_SESSION_SECRET` - Session security

### Optional Variables
- `OPENAI_API_KEY` - For embeddings and RAG
- `QDRANT_API_KEY` - For secured Qdrant access
- `QDRANT_URL` - Custom Qdrant endpoint
- `TIKA_URL` - Custom Tika endpoint
- `OTEL_EXPORTER_OTLP_ENDPOINT` - For observability

### Validation Rules
- `APP_ENV` must be one of: `dev`, `test`, `prod`
- `PORT` must be a valid port number (1-65535)
- `MAX_UPLOAD_MB` must be a positive integer
- `RATE_LIMIT_PER_MIN` must be a positive integer
- `DATA_RETENTION_DAYS` must be a positive integer

## Security Considerations

### Sensitive Variables
These variables contain sensitive information and should be encrypted with SOPS:

- `OPENROUTER_API_KEY`
- `OPENAI_API_KEY`
- `QDRANT_API_KEY`
- `ADMIN_SESSION_SECRET`
- `UNIDOC_LICENSE_API_KEY`
- `DB_URL` (contains credentials)

### Environment-Specific Secrets
- Development: Use `.env.sops.yaml`
- Production: Use `.env.production.sops.yaml`

## Troubleshooting

### Common Issues

1. **Database Connection Failed**
   ```bash
   # Check DB_URL format
   echo $DB_URL
   # Should be: postgres://user:password@host:port/database?sslmode=disable
   ```

2. **Queue Connection Failed**
   ```bash
   # Check KAFKA_BROKERS
   echo $KAFKA_BROKERS
   # Should be: host1:port1,host2:port2,host3:port3
   ```

3. **AI API Errors**
   ```bash
   # Check API keys
   echo $OPENROUTER_API_KEY
   echo $OPENAI_API_KEY
   ```

4. **Frontend Not Loading**
   ```bash
   # Check CORS configuration
   echo $CORS_ALLOW_ORIGINS
   # Should include your frontend URL
   ```

### Validation Commands
```bash
# Validate environment variables
go run ./cmd/server --validate-config

# Check required variables
env | grep -E "(DB_URL|KAFKA_BROKERS|OPENROUTER_API_KEY)"

# Test database connection
pg_isready -d "$DB_URL"

# Test queue connection
kafka-topics --bootstrap-server "$KAFKA_BROKERS" --list
```

## Best Practices

### 1. Environment Separation
- Use different `.env` files for different environments
- Never commit `.env` files to version control
- Use SOPS for encrypting sensitive variables

### 2. Default Values
- Always provide sensible defaults
- Use environment-specific overrides
- Document all default values

### 3. Validation
- Validate all environment variables at startup
- Provide clear error messages for invalid values
- Use type-safe configuration loading

### 4. Security
- Encrypt sensitive variables with SOPS
- Use different secrets per environment
- Rotate secrets regularly

### 5. Documentation
- Keep this document updated with new variables
- Include examples for all environments
- Document validation rules and requirements

---

*This document should be updated whenever new environment variables are added or existing ones are modified.*
