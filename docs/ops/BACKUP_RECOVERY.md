# Backup and Recovery Procedures

This document outlines comprehensive backup and recovery procedures for the AI CV Evaluator system, ensuring data protection and business continuity.

## Overview

The backup and recovery strategy protects against data loss, system failures, and disasters while ensuring rapid restoration of services.

## Backup Strategy

### 1. Backup Types

#### 1.1 Database Backups
```bash
#!/bin/bash
# Database backup script

# Set backup variables
BACKUP_DIR="/opt/backups/database"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="ai_cv_evaluator_${DATE}.sql"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Create database backup
echo "Creating database backup..."
docker exec ai-cv-evaluator-db pg_dump -U postgres -h localhost app > "$BACKUP_PATH"

# Compress backup
echo "Compressing backup..."
gzip "$BACKUP_PATH"
BACKUP_PATH="${BACKUP_PATH}.gz"

# Verify backup
echo "Verifying backup..."
if [ -f "$BACKUP_PATH" ]; then
    echo "Backup created successfully: $BACKUP_PATH"
    echo "Backup size: $(du -h "$BACKUP_PATH" | cut -f1)"
else
    echo "Backup failed!"
    exit 1
fi

# Clean up old backups (keep 30 days)
echo "Cleaning up old backups..."
find "$BACKUP_DIR" -name "*.sql.gz" -mtime +30 -delete

echo "Database backup completed"
```

#### 1.2 Configuration Backups
```bash
#!/bin/bash
# Configuration backup script

# Set backup variables
BACKUP_DIR="/opt/backups/configuration"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="configuration_${DATE}.tar.gz"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Backup configuration files
echo "Creating configuration backup..."
tar -czf "$BACKUP_PATH" \
    docker-compose.yml \
    docker-compose.prod.yml \
    .env \
    .env.production \
    deploy/ \
    configs/ \
    secrets/

# Verify backup
echo "Verifying backup..."
if [ -f "$BACKUP_PATH" ]; then
    echo "Configuration backup created successfully: $BACKUP_PATH"
    echo "Backup size: $(du -h "$BACKUP_PATH" | cut -f1)"
else
    echo "Configuration backup failed!"
    exit 1
fi

# Clean up old backups (keep 90 days)
echo "Cleaning up old backups..."
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +90 -delete

echo "Configuration backup completed"
```

#### 1.3 Application Data Backups
```bash
#!/bin/bash
# Application data backup script

# Set backup variables
BACKUP_DIR="/opt/backups/application"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="application_data_${DATE}.tar.gz"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Backup application data
echo "Creating application data backup..."
tar -czf "$BACKUP_PATH" \
    /var/lib/docker/volumes/ai-cv-evaluator_db_data \
    /var/lib/docker/volumes/ai-cv-evaluator_redpanda_data \
    /var/lib/docker/volumes/ai-cv-evaluator_qdrant_data \
    /opt/ai-cv-evaluator/logs/ \
    /opt/ai-cv-evaluator/artifacts/

# Verify backup
echo "Verifying backup..."
if [ -f "$BACKUP_PATH" ]; then
    echo "Application data backup created successfully: $BACKUP_PATH"
    echo "Backup size: $(du -h "$BACKUP_PATH" | cut -f1)"
else
    echo "Application data backup failed!"
    exit 1
fi

# Clean up old backups (keep 30 days)
echo "Cleaning up old backups..."
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete

echo "Application data backup completed"
```

### 2. Backup Schedule

#### 2.1 Automated Backup Schedule
```bash
# Crontab entries for automated backups
# Daily database backup at 2 AM
0 2 * * * /opt/ai-cv-evaluator/scripts/backup-database.sh

# Daily configuration backup at 2:30 AM
30 2 * * * /opt/ai-cv-evaluator/scripts/backup-configuration.sh

# Daily application data backup at 3 AM
0 3 * * * /opt/ai-cv-evaluator/scripts/backup-application-data.sh

# Weekly full system backup on Sunday at 4 AM
0 4 * * 0 /opt/ai-cv-evaluator/scripts/backup-full-system.sh

# Monthly archive backup on 1st at 5 AM
0 5 1 * * /opt/ai-cv-evaluator/scripts/backup-archive.sh
```

#### 2.2 Backup Retention Policy
```yaml
# Backup retention policy
database_backups:
  daily: 30 days
  weekly: 12 weeks
  monthly: 12 months
  yearly: 7 years

configuration_backups:
  daily: 90 days
  weekly: 52 weeks
  monthly: 24 months

application_data_backups:
  daily: 30 days
  weekly: 12 weeks
  monthly: 12 months

full_system_backups:
  weekly: 12 weeks
  monthly: 12 months
  yearly: 7 years
```

### 3. Backup Verification

#### 3.1 Backup Integrity Checks
```bash
#!/bin/bash
# Backup verification script

echo "=== Backup Verification ==="

# Check database backup integrity
echo "Verifying database backup..."
LATEST_DB_BACKUP=$(ls -t /opt/backups/database/*.sql.gz | head -1)
if [ -f "$LATEST_DB_BACKUP" ]; then
    echo "Testing database backup: $LATEST_DB_BACKUP"
    gunzip -t "$LATEST_DB_BACKUP" && echo "Database backup is valid" || echo "Database backup is corrupted"
else
    echo "No database backup found"
fi

# Check configuration backup integrity
echo "Verifying configuration backup..."
LATEST_CONFIG_BACKUP=$(ls -t /opt/backups/configuration/*.tar.gz | head -1)
if [ -f "$LATEST_CONFIG_BACKUP" ]; then
    echo "Testing configuration backup: $LATEST_CONFIG_BACKUP"
    tar -tzf "$LATEST_CONFIG_BACKUP" > /dev/null && echo "Configuration backup is valid" || echo "Configuration backup is corrupted"
else
    echo "No configuration backup found"
fi

# Check application data backup integrity
echo "Verifying application data backup..."
LATEST_APP_BACKUP=$(ls -t /opt/backups/application/*.tar.gz | head -1)
if [ -f "$LATEST_APP_BACKUP" ]; then
    echo "Testing application data backup: $LATEST_APP_BACKUP"
    tar -tzf "$LATEST_APP_BACKUP" > /dev/null && echo "Application data backup is valid" || echo "Application data backup is corrupted"
else
    echo "No application data backup found"
fi

echo "Backup verification completed"
```

#### 3.2 Backup Testing
```bash
#!/bin/bash
# Backup testing script

echo "=== Backup Testing ==="

# Test database backup restoration
echo "Testing database backup restoration..."
LATEST_DB_BACKUP=$(ls -t /opt/backups/database/*.sql.gz | head -1)
if [ -f "$LATEST_DB_BACKUP" ]; then
    # Create test database
    docker exec ai-cv-evaluator-db psql -U postgres -c "CREATE DATABASE test_restore;"
    
    # Restore backup to test database
    gunzip -c "$LATEST_DB_BACKUP" | docker exec -i ai-cv-evaluator-db psql -U postgres -d test_restore
    
    # Verify restoration
    TABLE_COUNT=$(docker exec ai-cv-evaluator-db psql -U postgres -d test_restore -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" -t)
    if [ "$TABLE_COUNT" -gt 0 ]; then
        echo "Database backup restoration test passed"
    else
        echo "Database backup restoration test failed"
    fi
    
    # Clean up test database
    docker exec ai-cv-evaluator-db psql -U postgres -c "DROP DATABASE test_restore;"
else
    echo "No database backup found for testing"
fi

echo "Backup testing completed"
```

## Recovery Procedures

### 1. Database Recovery

#### 1.1 Full Database Recovery
```bash
#!/bin/bash
# Database recovery script

# Set recovery variables
BACKUP_DIR="/opt/backups/database"
BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file>"
    echo "Available backups:"
    ls -la "$BACKUP_DIR"/*.sql.gz
    exit 1
fi

BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Verify backup exists
if [ ! -f "$BACKUP_PATH" ]; then
    echo "Backup file not found: $BACKUP_PATH"
    exit 1
fi

echo "=== Database Recovery ==="
echo "Recovering from backup: $BACKUP_PATH"

# Stop application services
echo "Stopping application services..."
docker compose stop app worker

# Create database backup before recovery
echo "Creating pre-recovery backup..."
docker exec ai-cv-evaluator-db pg_dump -U postgres -h localhost app > "/opt/backups/database/pre_recovery_$(date +%Y%m%d_%H%M%S).sql"

# Drop and recreate database
echo "Dropping and recreating database..."
docker exec ai-cv-evaluator-db psql -U postgres -c "DROP DATABASE IF EXISTS app;"
docker exec ai-cv-evaluator-db psql -U postgres -c "CREATE DATABASE app;"

# Restore database
echo "Restoring database..."
gunzip -c "$BACKUP_PATH" | docker exec -i ai-cv-evaluator-db psql -U postgres -d app

# Verify restoration
echo "Verifying restoration..."
TABLE_COUNT=$(docker exec ai-cv-evaluator-db psql -U postgres -d app -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" -t)
if [ "$TABLE_COUNT" -gt 0 ]; then
    echo "Database recovery successful"
    echo "Tables restored: $TABLE_COUNT"
else
    echo "Database recovery failed"
    exit 1
fi

# Start application services
echo "Starting application services..."
docker compose start app worker

# Verify application health
echo "Verifying application health..."
sleep 30
curl -f http://localhost:8080/healthz && echo "Application is healthy" || echo "Application health check failed"

echo "Database recovery completed"
```

#### 1.2 Point-in-Time Recovery
```bash
#!/bin/bash
# Point-in-time recovery script

# Set recovery variables
BACKUP_DIR="/opt/backups/database"
BACKUP_FILE="$1"
RECOVERY_TIME="$2"

if [ -z "$BACKUP_FILE" ] || [ -z "$RECOVERY_TIME" ]; then
    echo "Usage: $0 <backup_file> <recovery_time>"
    echo "Example: $0 ai_cv_evaluator_20240115_020000.sql.gz '2024-01-15 02:00:00'"
    exit 1
fi

BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Verify backup exists
if [ ! -f "$BACKUP_PATH" ]; then
    echo "Backup file not found: $BACKUP_PATH"
    exit 1
fi

echo "=== Point-in-Time Recovery ==="
echo "Recovering to: $RECOVERY_TIME"
echo "From backup: $BACKUP_PATH"

# Stop application services
echo "Stopping application services..."
docker compose stop app worker

# Create database backup before recovery
echo "Creating pre-recovery backup..."
docker exec ai-cv-evaluator-db pg_dump -U postgres -h localhost app > "/opt/backups/database/pre_recovery_$(date +%Y%m%d_%H%M%S).sql"

# Drop and recreate database
echo "Dropping and recreating database..."
docker exec ai-cv-evaluator-db psql -U postgres -c "DROP DATABASE IF EXISTS app;"
docker exec ai-cv-evaluator-db psql -U postgres -c "CREATE DATABASE app;"

# Restore database
echo "Restoring database..."
gunzip -c "$BACKUP_PATH" | docker exec -i ai-cv-evaluator-db psql -U postgres -d app

# Apply WAL files for point-in-time recovery
echo "Applying WAL files for point-in-time recovery..."
# Note: This requires WAL archiving to be enabled
# docker exec ai-cv-evaluator-db pg_recovery -D /var/lib/postgresql/data -t "$RECOVERY_TIME"

# Verify restoration
echo "Verifying restoration..."
TABLE_COUNT=$(docker exec ai-cv-evaluator-db psql -U postgres -d app -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" -t)
if [ "$TABLE_COUNT" -gt 0 ]; then
    echo "Point-in-time recovery successful"
    echo "Tables restored: $TABLE_COUNT"
else
    echo "Point-in-time recovery failed"
    exit 1
fi

# Start application services
echo "Starting application services..."
docker compose start app worker

echo "Point-in-time recovery completed"
```

### 2. Configuration Recovery

#### 2.1 Configuration Restoration
```bash
#!/bin/bash
# Configuration recovery script

# Set recovery variables
BACKUP_DIR="/opt/backups/configuration"
BACKUP_FILE="$1"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file>"
    echo "Available backups:"
    ls -la "$BACKUP_DIR"/*.tar.gz
    exit 1
fi

BACKUP_PATH="${BACKUP_DIR}/${BACKUP_FILE}"

# Verify backup exists
if [ ! -f "$BACKUP_PATH" ]; then
    echo "Backup file not found: $BACKUP_PATH"
    exit 1
fi

echo "=== Configuration Recovery ==="
echo "Recovering from backup: $BACKUP_PATH"

# Stop all services
echo "Stopping all services..."
docker compose down

# Create configuration backup before recovery
echo "Creating pre-recovery configuration backup..."
tar -czf "/opt/backups/configuration/pre_recovery_$(date +%Y%m%d_%H%M%S).tar.gz" \
    docker-compose.yml \
    docker-compose.prod.yml \
    .env \
    .env.production \
    deploy/ \
    configs/ \
    secrets/

# Restore configuration
echo "Restoring configuration..."
tar -xzf "$BACKUP_PATH"

# Verify restoration
echo "Verifying configuration restoration..."
if [ -f "docker-compose.yml" ] && [ -f ".env" ]; then
    echo "Configuration recovery successful"
else
    echo "Configuration recovery failed"
    exit 1
fi

# Start services
echo "Starting services..."
docker compose up -d

# Verify services
echo "Verifying services..."
sleep 30
curl -f http://localhost:8080/healthz && echo "Services are healthy" || echo "Service health check failed"

echo "Configuration recovery completed"
```

### 3. Full System Recovery

#### 3.1 Complete System Recovery
```bash
#!/bin/bash
# Full system recovery script

# Set recovery variables
BACKUP_DIR="/opt/backups"
DB_BACKUP="$1"
CONFIG_BACKUP="$2"
APP_BACKUP="$3"

if [ -z "$DB_BACKUP" ] || [ -z "$CONFIG_BACKUP" ] || [ -z "$APP_BACKUP" ]; then
    echo "Usage: $0 <db_backup> <config_backup> <app_backup>"
    echo "Available backups:"
    echo "Database:"
    ls -la "$BACKUP_DIR/database"/*.sql.gz
    echo "Configuration:"
    ls -la "$BACKUP_DIR/configuration"/*.tar.gz
    echo "Application:"
    ls -la "$BACKUP_DIR/application"/*.tar.gz
    exit 1
fi

echo "=== Full System Recovery ==="
echo "Database backup: $DB_BACKUP"
echo "Configuration backup: $CONFIG_BACKUP"
echo "Application backup: $APP_BACKUP"

# Stop all services
echo "Stopping all services..."
docker compose down

# Remove existing data
echo "Removing existing data..."
docker volume rm ai-cv-evaluator_db_data ai-cv-evaluator_redpanda_data ai-cv-evaluator_qdrant_data 2>/dev/null || true

# Restore configuration
echo "Restoring configuration..."
tar -xzf "$BACKUP_DIR/configuration/$CONFIG_BACKUP"

# Restore application data
echo "Restoring application data..."
tar -xzf "$BACKUP_DIR/application/$APP_BACKUP"

# Start services
echo "Starting services..."
docker compose up -d

# Wait for services to be ready
echo "Waiting for services to be ready..."
sleep 60

# Restore database
echo "Restoring database..."
gunzip -c "$BACKUP_DIR/database/$DB_BACKUP" | docker exec -i ai-cv-evaluator-db psql -U postgres -d app

# Verify restoration
echo "Verifying restoration..."
curl -f http://localhost:8080/healthz && echo "System recovery successful" || echo "System recovery failed"

echo "Full system recovery completed"
```

### 4. Disaster Recovery

#### 4.1 Disaster Recovery Procedures
```bash
#!/bin/bash
# Disaster recovery script

echo "=== Disaster Recovery ==="

# Check system status
echo "Checking system status..."
if curl -f http://localhost:8080/healthz 2>/dev/null; then
    echo "System is running normally"
    exit 0
fi

echo "System is down, initiating disaster recovery..."

# Check backup availability
echo "Checking backup availability..."
LATEST_DB_BACKUP=$(ls -t /opt/backups/database/*.sql.gz | head -1)
LATEST_CONFIG_BACKUP=$(ls -t /opt/backups/configuration/*.tar.gz | head -1)
LATEST_APP_BACKUP=$(ls -t /opt/backups/application/*.tar.gz | head -1)

if [ ! -f "$LATEST_DB_BACKUP" ] || [ ! -f "$LATEST_CONFIG_BACKUP" ] || [ ! -f "$LATEST_APP_BACKUP" ]; then
    echo "Required backups not found"
    echo "Database: $LATEST_DB_BACKUP"
    echo "Configuration: $LATEST_CONFIG_BACKUP"
    echo "Application: $LATEST_APP_BACKUP"
    exit 1
fi

# Perform full system recovery
echo "Performing full system recovery..."
./scripts/recover-full-system.sh \
    "$(basename "$LATEST_DB_BACKUP")" \
    "$(basename "$LATEST_CONFIG_BACKUP")" \
    "$(basename "$LATEST_APP_BACKUP")"

# Verify recovery
echo "Verifying recovery..."
sleep 30
if curl -f http://localhost:8080/healthz; then
    echo "Disaster recovery successful"
else
    echo "Disaster recovery failed"
    exit 1
fi

echo "Disaster recovery completed"
```

#### 4.2 Recovery Testing
```bash
#!/bin/bash
# Recovery testing script

echo "=== Recovery Testing ==="

# Test database recovery
echo "Testing database recovery..."
LATEST_DB_BACKUP=$(ls -t /opt/backups/database/*.sql.gz | head -1)
if [ -f "$LATEST_DB_BACKUP" ]; then
    ./scripts/recover-database.sh "$(basename "$LATEST_DB_BACKUP")"
    if [ $? -eq 0 ]; then
        echo "Database recovery test passed"
    else
        echo "Database recovery test failed"
    fi
else
    echo "No database backup found for testing"
fi

# Test configuration recovery
echo "Testing configuration recovery..."
LATEST_CONFIG_BACKUP=$(ls -t /opt/backups/configuration/*.tar.gz | head -1)
if [ -f "$LATEST_CONFIG_BACKUP" ]; then
    ./scripts/recover-configuration.sh "$(basename "$LATEST_CONFIG_BACKUP")"
    if [ $? -eq 0 ]; then
        echo "Configuration recovery test passed"
    else
        echo "Configuration recovery test failed"
    fi
else
    echo "No configuration backup found for testing"
fi

# Test full system recovery
echo "Testing full system recovery..."
LATEST_APP_BACKUP=$(ls -t /opt/backups/application/*.tar.gz | head -1)
if [ -f "$LATEST_APP_BACKUP" ]; then
    ./scripts/recover-full-system.sh \
        "$(basename "$LATEST_DB_BACKUP")" \
        "$(basename "$LATEST_CONFIG_BACKUP")" \
        "$(basename "$LATEST_APP_BACKUP")"
    if [ $? -eq 0 ]; then
        echo "Full system recovery test passed"
    else
        echo "Full system recovery test failed"
    fi
else
    echo "No application backup found for testing"
fi

echo "Recovery testing completed"
```

## Backup Monitoring

### 1. Backup Status Monitoring

#### 1.1 Backup Health Checks
```bash
#!/bin/bash
# Backup health check script

echo "=== Backup Health Check ==="

# Check database backups
echo "Checking database backups..."
DB_BACKUP_COUNT=$(ls /opt/backups/database/*.sql.gz 2>/dev/null | wc -l)
if [ "$DB_BACKUP_COUNT" -gt 0 ]; then
    LATEST_DB_BACKUP=$(ls -t /opt/backups/database/*.sql.gz | head -1)
    DB_BACKUP_AGE=$(($(date +%s) - $(stat -c %Y "$LATEST_DB_BACKUP")))
    DB_BACKUP_AGE_HOURS=$((DB_BACKUP_AGE / 3600))
    echo "Database backups: $DB_BACKUP_COUNT (latest: $DB_BACKUP_AGE_HOURS hours ago)"
    
    if [ "$DB_BACKUP_AGE_HOURS" -gt 25 ]; then
        echo "WARNING: Database backup is older than 25 hours"
    fi
else
    echo "ERROR: No database backups found"
fi

# Check configuration backups
echo "Checking configuration backups..."
CONFIG_BACKUP_COUNT=$(ls /opt/backups/configuration/*.tar.gz 2>/dev/null | wc -l)
if [ "$CONFIG_BACKUP_COUNT" -gt 0 ]; then
    LATEST_CONFIG_BACKUP=$(ls -t /opt/backups/configuration/*.tar.gz | head -1)
    CONFIG_BACKUP_AGE=$(($(date +%s) - $(stat -c %Y "$LATEST_CONFIG_BACKUP")))
    CONFIG_BACKUP_AGE_HOURS=$((CONFIG_BACKUP_AGE / 3600))
    echo "Configuration backups: $CONFIG_BACKUP_COUNT (latest: $CONFIG_BACKUP_AGE_HOURS hours ago)"
    
    if [ "$CONFIG_BACKUP_AGE_HOURS" -gt 25 ]; then
        echo "WARNING: Configuration backup is older than 25 hours"
    fi
else
    echo "ERROR: No configuration backups found"
fi

# Check application backups
echo "Checking application backups..."
APP_BACKUP_COUNT=$(ls /opt/backups/application/*.tar.gz 2>/dev/null | wc -l)
if [ "$APP_BACKUP_COUNT" -gt 0 ]; then
    LATEST_APP_BACKUP=$(ls -t /opt/backups/application/*.tar.gz | head -1)
    APP_BACKUP_AGE=$(($(date +%s) - $(stat -c %Y "$LATEST_APP_BACKUP")))
    APP_BACKUP_AGE_HOURS=$((APP_BACKUP_AGE / 3600))
    echo "Application backups: $APP_BACKUP_COUNT (latest: $APP_BACKUP_AGE_HOURS hours ago)"
    
    if [ "$APP_BACKUP_AGE_HOURS" -gt 25 ]; then
        echo "WARNING: Application backup is older than 25 hours"
    fi
else
    echo "ERROR: No application backups found"
fi

echo "Backup health check completed"
```

#### 1.2 Backup Alerting
```yaml
# Prometheus alerting rules for backups
groups:
  - name: backup
    rules:
      - alert: BackupMissing
        expr: time() - backup_last_success_time > 86400
        for: 1h
        labels:
          severity: critical
        annotations:
          summary: "Backup missing for more than 24 hours"
          description: "Last successful backup was {{ $value }} seconds ago"

      - alert: BackupFailed
        expr: backup_success == 0
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Backup failed"
          description: "Backup process failed"

      - alert: BackupCorrupted
        expr: backup_integrity_check == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Backup is corrupted"
          description: "Backup integrity check failed"
```

### 2. Backup Reporting

#### 2.1 Backup Reports
```bash
#!/bin/bash
# Backup report script

echo "=== Backup Report ==="
echo "Date: $(date)"
echo "System: AI CV Evaluator"
echo ""

# Database backup report
echo "=== Database Backups ==="
DB_BACKUP_COUNT=$(ls /opt/backups/database/*.sql.gz 2>/dev/null | wc -l)
if [ "$DB_BACKUP_COUNT" -gt 0 ]; then
    echo "Total backups: $DB_BACKUP_COUNT"
    echo "Latest backup: $(ls -t /opt/backups/database/*.sql.gz | head -1)"
    echo "Backup size: $(du -sh /opt/backups/database/ | cut -f1)"
    echo "Oldest backup: $(ls -t /opt/backups/database/*.sql.gz | tail -1)"
else
    echo "No database backups found"
fi
echo ""

# Configuration backup report
echo "=== Configuration Backups ==="
CONFIG_BACKUP_COUNT=$(ls /opt/backups/configuration/*.tar.gz 2>/dev/null | wc -l)
if [ "$CONFIG_BACKUP_COUNT" -gt 0 ]; then
    echo "Total backups: $CONFIG_BACKUP_COUNT"
    echo "Latest backup: $(ls -t /opt/backups/configuration/*.tar.gz | head -1)"
    echo "Backup size: $(du -sh /opt/backups/configuration/ | cut -f1)"
    echo "Oldest backup: $(ls -t /opt/backups/configuration/*.tar.gz | tail -1)"
else
    echo "No configuration backups found"
fi
echo ""

# Application backup report
echo "=== Application Backups ==="
APP_BACKUP_COUNT=$(ls /opt/backups/application/*.tar.gz 2>/dev/null | wc -l)
if [ "$APP_BACKUP_COUNT" -gt 0 ]; then
    echo "Total backups: $APP_BACKUP_COUNT"
    echo "Latest backup: $(ls -t /opt/backups/application/*.tar.gz | head -1)"
    echo "Backup size: $(du -sh /opt/backups/application/ | cut -f1)"
    echo "Oldest backup: $(ls -t /opt/backups/application/*.tar.gz | tail -1)"
else
    echo "No application backups found"
fi
echo ""

# Backup integrity report
echo "=== Backup Integrity ==="
echo "Database backups:"
for backup in /opt/backups/database/*.sql.gz; do
    if [ -f "$backup" ]; then
        if gunzip -t "$backup" 2>/dev/null; then
            echo "  $(basename "$backup"): OK"
        else
            echo "  $(basename "$backup"): CORRUPTED"
        fi
    fi
done

echo "Configuration backups:"
for backup in /opt/backups/configuration/*.tar.gz; do
    if [ -f "$backup" ]; then
        if tar -tzf "$backup" > /dev/null 2>&1; then
            echo "  $(basename "$backup"): OK"
        else
            echo "  $(basename "$backup"): CORRUPTED"
        fi
    fi
done

echo "Application backups:"
for backup in /opt/backups/application/*.tar.gz; do
    if [ -f "$backup" ]; then
        if tar -tzf "$backup" > /dev/null 2>&1; then
            echo "  $(basename "$backup"): OK"
        else
            echo "  $(basename "$backup"): CORRUPTED"
        fi
    fi
done

echo "Backup report completed"
```

## Backup Best Practices

### 1. Backup Strategy

#### 1.1 3-2-1 Rule
- **3 copies** of data (original + 2 backups)
- **2 different media** types (local + remote)
- **1 offsite** backup (cloud storage)

#### 1.2 Backup Types
- **Full backups**: Complete system backup
- **Incremental backups**: Changes since last backup
- **Differential backups**: Changes since last full backup
- **Snapshot backups**: Point-in-time system state

### 2. Backup Security

#### 2.1 Encryption
```bash
# Encrypt backups before storage
gpg --symmetric --cipher-algo AES256 --compress-algo 1 --s2k-mode 3 --s2k-digest-algo SHA512 --s2k-count 65536 "$BACKUP_PATH"
```

#### 2.2 Access Control
```bash
# Set proper permissions on backup files
chmod 600 /opt/backups/database/*.sql.gz
chmod 600 /opt/backups/configuration/*.tar.gz
chmod 600 /opt/backups/application/*.tar.gz

# Set backup directory ownership
chown -R backup:backup /opt/backups/
```

### 3. Recovery Testing

#### 3.1 Regular Testing
- **Monthly**: Test database recovery
- **Quarterly**: Test full system recovery
- **Annually**: Test disaster recovery procedures

#### 3.2 Recovery Validation
- **Data integrity**: Verify all data is restored
- **System functionality**: Test all critical features
- **Performance**: Ensure system performance is maintained
- **Security**: Verify security configurations are restored

---

*This backup and recovery guide ensures comprehensive data protection and rapid system restoration for the AI CV Evaluator project.*
