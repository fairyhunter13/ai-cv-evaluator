# Database Migration System

This document describes the containerized database migration system used in the AI CV Evaluator project.

## Overview

Database migrations are handled by a dedicated Docker container that runs automatically when starting the application stack. This ensures consistent migration execution across development, testing, and production environments.

## Architecture

### Migration Container (`Dockerfile.migrate`)

The migration system uses a multi-stage Docker build:

1. **Builder Stage**: Uses `golang:1.24-alpine` to install `goose` migration tool
2. **Runtime Stage**: Uses `alpine:3.19` with minimal dependencies:
   - `goose` binary (migration tool)
   - `postgresql-client` (for database connectivity)
   - Migration files from `deploy/migrations/`

### Automatic Execution

Migrations run automatically via Docker Compose service dependencies:

```yaml
# Migration service runs first
migrate:
  depends_on:
    db:
      condition: service_healthy
  restart: "no"  # Runs once and exits

# App services wait for migrations to complete
app:
  depends_on:
    migrate:
      condition: service_completed_successfully
```

## Migration Files

- **Location**: `deploy/migrations/`
- **Format**: SQL files with goose annotations
- **Naming**: Timestamped format (e.g., `20250927122000_init.sql`)
- **Structure**: Each file contains `+goose Up` and `+goose Down` sections

### Example Migration File

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS uploads (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL CHECK (type IN ('cv','project')),
  text TEXT NOT NULL,
  filename TEXT NOT NULL,
  mime TEXT NOT NULL,
  size BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS uploads;
-- +goose StatementEnd
```

## Usage

### Development

```bash
# Migrations run automatically
docker compose up -d

# Or run manually
make migrate
```

### Production

```bash
# Migrations run automatically
docker compose -f docker-compose.prod.yml up -d

# Or run manually
docker compose -f docker-compose.prod.yml run --rm migrate
```

### CI/CD

Migrations are handled automatically in GitHub Actions:

1. **Build Stage**: Migration container is built and pushed to GHCR
2. **Deploy Stage**: Migration container runs before app services start
3. **Rollback**: Automatic rollback if migrations fail

## Features

### Robust Error Handling

The migration container includes:

- **Database Health Checks**: Waits for database to be ready
- **Retry Logic**: Automatically retries failed migrations after 5 seconds
- **Connection Validation**: Verifies database connectivity before running migrations
- **Exit Codes**: Proper exit codes for success/failure detection

### Environment Variables

- `DB_URL`: PostgreSQL connection string (required)
- Automatically loaded from `.env` or `.env.production` files

### Logging

The migration container provides detailed logging:

```
==> Running database migrations...
==> Waiting for database to be ready...
==> Database is ready, running migrations...
==> Migrations completed successfully
```

## Benefits

### Consistency
- Same migration environment across all deployment stages
- No dependency on local development tools
- Consistent goose version and configuration

### Reliability
- Automatic retry on failure
- Database health checks before migration
- Proper error handling and exit codes

### Security
- No need to install migration tools locally
- Encrypted secrets support via SOPS
- Containerized execution environment

### Automation
- Zero-touch migration execution
- Integration with Docker Compose lifecycle
- CI/CD pipeline integration

## Troubleshooting

### Migration Failures

1. **Check Database Connectivity**:
   ```bash
   docker compose exec db pg_isready -U postgres
   ```

2. **View Migration Logs**:
   ```bash
   docker compose logs migrate
   ```

3. **Run Migrations Manually**:
   ```bash
   make migrate
   ```

### Common Issues

- **Database Not Ready**: Ensure PostgreSQL is healthy before migrations
- **Permission Issues**: Check database user permissions
- **Network Issues**: Verify container networking
- **Migration Conflicts**: Check for concurrent migration attempts

## Migration Best Practices

### File Naming
- Use timestamp format: `YYYYMMDDHHMMSS_description.sql`
- Keep descriptions concise but descriptive
- Avoid special characters in filenames

### SQL Structure
- Always include both `Up` and `Down` migrations
- Use transactions for complex migrations
- Test migrations in development first
- Keep migrations idempotent when possible

### Testing
- Test migrations in development environment
- Verify rollback scenarios
- Test with production-like data volumes
- Validate migration performance

## Integration Points

### Docker Compose
- Development: `docker-compose.yml`
- Production: `docker-compose.prod.yml`
- Both include migration service with proper dependencies

### Makefile
- `make migrate`: Run migrations manually
- `make docker-build`: Build migration container
- `make ci-e2e`: Test with automatic migrations

### CI/CD
- GitHub Actions workflows build and deploy migration container
- Automatic execution during deployment
- Rollback on migration failure

## Future Enhancements

- **Migration Validation**: Pre-migration schema validation
- **Backup Integration**: Automatic database backups before migration
- **Performance Monitoring**: Migration execution time tracking
- **Rollback Automation**: Automatic rollback on application failure
