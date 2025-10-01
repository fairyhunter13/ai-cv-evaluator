# Maintenance Procedures

This document outlines comprehensive maintenance procedures for the AI CV Evaluator system, including routine maintenance, preventive measures, and system optimization.

## Overview

Regular maintenance ensures system reliability, performance, and security while minimizing downtime and preventing issues before they occur.

## Maintenance Schedule

### 1. Daily Maintenance

#### 1.1 System Health Checks
```bash
#!/bin/bash
# Daily maintenance script

# Check system health
echo "=== Daily Health Check ==="
curl -f http://localhost:8080/healthz || echo "Health check failed"
curl -f http://localhost:8080/readyz || echo "Readiness check failed"

# Check resource usage
echo "=== Resource Usage ==="
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# Check disk space
echo "=== Disk Space ==="
df -h

# Check log sizes
echo "=== Log Sizes ==="
du -sh /var/log/* 2>/dev/null | sort -hr | head -10
```

#### 1.2 Database Maintenance
```sql
-- Daily database maintenance
-- Check database health
SELECT 
    datname,
    numbackends,
    xact_commit,
    xact_rollback,
    blks_read,
    blks_hit
FROM pg_stat_database 
WHERE datname = 'app';

-- Check for long-running queries
SELECT 
    pid,
    now() - pg_stat_activity.query_start AS duration,
    query
FROM pg_stat_activity 
WHERE (now() - pg_stat_activity.query_start) > interval '5 minutes';

-- Check database size
SELECT 
    pg_size_pretty(pg_database_size('app')) AS database_size;
```

#### 1.3 Queue Maintenance
```bash
# Check queue health
echo "=== Queue Health ==="
rpk cluster health

# Check topic status
echo "=== Topic Status ==="
rpk topic describe evaluate-jobs

# Check consumer group lag
echo "=== Consumer Group Status ==="
rpk group describe ai-cv-evaluator-workers
```

### 2. Weekly Maintenance

#### 2.1 System Updates
```bash
#!/bin/bash
# Weekly maintenance script

# Update system packages
echo "=== System Updates ==="
apt update && apt upgrade -y

# Update Docker images
echo "=== Docker Image Updates ==="
docker compose pull

# Check for security updates
echo "=== Security Updates ==="
apt list --upgradable | grep -E "(security|critical)"
```

#### 2.2 Database Optimization
```sql
-- Weekly database maintenance
-- Analyze tables for query optimization
ANALYZE;

-- Update table statistics
ANALYZE uploads;
ANALYZE jobs;
ANALYZE results;

-- Check for table bloat
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    pg_size_pretty(pg_relation_size(schemaname||'.'||tablename)) AS table_size,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) AS index_size
FROM pg_tables 
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

#### 2.3 Log Rotation and Cleanup
```bash
# Log rotation
echo "=== Log Rotation ==="
logrotate -f /etc/logrotate.conf

# Clean up old logs
echo "=== Log Cleanup ==="
find /var/log -name "*.log" -mtime +30 -delete
find /var/log -name "*.gz" -mtime +90 -delete

# Clean up Docker logs
echo "=== Docker Log Cleanup ==="
docker system prune -f
docker volume prune -f
```

### 3. Monthly Maintenance

#### 3.1 Security Updates
```bash
#!/bin/bash
# Monthly security maintenance

# Update all packages
echo "=== Package Updates ==="
apt update && apt upgrade -y

# Check for security vulnerabilities
echo "=== Security Scan ==="
make vuln

# Update Docker images
echo "=== Docker Updates ==="
docker compose pull
docker compose up -d --build

# Check for outdated dependencies
echo "=== Dependency Check ==="
go list -u -m all
npm outdated
```

#### 3.2 Database Maintenance
```sql
-- Monthly database maintenance
-- Vacuum and analyze all tables
VACUUM ANALYZE;

-- Check for unused indexes
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes 
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;

-- Check for table bloat
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    pg_stat_get_tuples_returned(c.oid) AS tuples_returned,
    pg_stat_get_tuples_fetched(c.oid) AS tuples_fetched
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = 'public' AND c.relkind = 'r'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

#### 3.3 Performance Optimization
```bash
# Performance analysis
echo "=== Performance Analysis ==="
# Check slow queries
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT query, calls, total_time, mean_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;"

# Check index usage
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT schemaname, tablename, indexname, idx_scan, idx_tup_read 
FROM pg_stat_user_indexes 
ORDER BY idx_scan DESC;"

# Check connection usage
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT state, count(*) 
FROM pg_stat_activity 
GROUP BY state;"
```

### 4. Quarterly Maintenance

#### 4.1 System Architecture Review
```bash
# Architecture review
echo "=== Architecture Review ==="
# Check system capacity
df -h
free -h
docker system df

# Check service dependencies
docker compose config

# Review configuration
echo "=== Configuration Review ==="
# Check environment variables
docker exec ai-cv-evaluator-app env | grep -E "(DB_|KAFKA_|AI_)"

# Check security configuration
docker exec ai-cv-evaluator-app cat /etc/ssl/certs/ca-certificates.crt | wc -l
```

#### 4.2 Disaster Recovery Testing
```bash
# Disaster recovery test
echo "=== Disaster Recovery Test ==="
# Test backup restoration
docker compose down
docker compose up -d db
# Restore from backup
# Verify data integrity
docker compose up -d

# Test failover procedures
echo "=== Failover Test ==="
# Simulate service failure
docker compose stop app
# Test failover mechanisms
# Restore service
docker compose start app
```

#### 4.3 Capacity Planning
```bash
# Capacity analysis
echo "=== Capacity Analysis ==="
# Check resource usage trends
curl -s http://localhost:9090/api/v1/query?query=rate(container_cpu_usage_seconds_total[7d])
curl -s http://localhost:9090/api/v1/query?query=container_memory_usage_bytes

# Check growth trends
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT 
    DATE(created_at) as date,
    COUNT(*) as daily_uploads
FROM uploads 
WHERE created_at > NOW() - INTERVAL '30 days'
GROUP BY DATE(created_at)
ORDER BY date;"
```

## Preventive Maintenance

### 1. System Monitoring

#### 1.1 Health Monitoring
```yaml
# Prometheus monitoring rules
groups:
  - name: maintenance
    rules:
      - alert: HighDiskUsage
        expr: (node_filesystem_avail_bytes / node_filesystem_size_bytes) < 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High disk usage detected"
          description: "Disk usage is {{ $value }}%"

      - alert: HighMemoryUsage
        expr: (container_memory_usage_bytes / container_spec_memory_limit_bytes) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage detected"
          description: "Memory usage is {{ $value }}%"

      - alert: DatabaseConnectionsHigh
        expr: pg_stat_activity_count > 80
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High database connection count"
          description: "Database connections: {{ $value }}"
```

#### 1.2 Performance Monitoring
```bash
# Performance monitoring script
#!/bin/bash
# Monitor key performance metrics

# Check response times
echo "=== Response Time Check ==="
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:8080/healthz

# Check queue performance
echo "=== Queue Performance ==="
rpk group describe ai-cv-evaluator-workers

# Check worker performance
echo "=== Worker Performance ==="
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"
```

### 2. Automated Maintenance

#### 2.1 Maintenance Scripts
```bash
#!/bin/bash
# Automated maintenance script

# Set maintenance mode
echo "=== Starting Maintenance ==="
curl -X POST http://localhost:8080/admin/maintenance/start

# Run maintenance tasks
echo "=== Running Maintenance Tasks ==="
# Database maintenance
docker exec ai-cv-evaluator-db psql -U postgres -c "VACUUM ANALYZE;"

# Log cleanup
find /var/log -name "*.log" -mtime +30 -delete

# Docker cleanup
docker system prune -f

# End maintenance mode
echo "=== Ending Maintenance ==="
curl -X POST http://localhost:8080/admin/maintenance/stop
```

#### 2.2 Scheduled Maintenance
```bash
# Crontab entries for automated maintenance
# Daily maintenance at 2 AM
0 2 * * * /opt/ai-cv-evaluator/scripts/daily-maintenance.sh

# Weekly maintenance on Sunday at 3 AM
0 3 * * 0 /opt/ai-cv-evaluator/scripts/weekly-maintenance.sh

# Monthly maintenance on 1st at 4 AM
0 4 1 * * /opt/ai-cv-evaluator/scripts/monthly-maintenance.sh
```

### 3. Maintenance Procedures

#### 3.1 Pre-Maintenance Checklist
```markdown
## Pre-Maintenance Checklist

### System Preparation
- [ ] Notify users of maintenance window
- [ ] Backup current system state
- [ ] Verify maintenance scripts
- [ ] Check system health before maintenance
- [ ] Prepare rollback procedures

### Maintenance Window
- [ ] Set maintenance mode
- [ ] Run maintenance tasks
- [ ] Verify system health
- [ ] Test critical functionality
- [ ] Clear maintenance mode

### Post-Maintenance
- [ ] Monitor system performance
- [ ] Verify all services are running
- [ ] Check for any issues
- [ ] Update documentation
- [ ] Notify users of completion
```

#### 3.2 Maintenance Execution
```bash
#!/bin/bash
# Maintenance execution script

# Set maintenance mode
echo "Setting maintenance mode..."
curl -X POST http://localhost:8080/admin/maintenance/start

# Wait for maintenance mode to be active
sleep 30

# Run maintenance tasks
echo "Running maintenance tasks..."
./scripts/database-maintenance.sh
./scripts/log-cleanup.sh
./scripts/security-updates.sh

# Verify system health
echo "Verifying system health..."
curl -f http://localhost:8080/healthz
curl -f http://localhost:8080/readyz

# Clear maintenance mode
echo "Clearing maintenance mode..."
curl -X POST http://localhost:8080/admin/maintenance/stop

# Monitor system for 10 minutes
echo "Monitoring system for 10 minutes..."
for i in {1..10}; do
    sleep 60
    curl -f http://localhost:8080/healthz || echo "Health check failed at minute $i"
done

echo "Maintenance completed successfully"
```

### 4. Maintenance Documentation

#### 4.1 Maintenance Log
```markdown
## Maintenance Log

### Date: 2024-01-15
### Type: Weekly Maintenance
### Duration: 2 hours
### Performed by: System Administrator

#### Tasks Completed
- [x] Database vacuum and analyze
- [x] Log rotation and cleanup
- [x] Security updates
- [x] Performance optimization
- [x] System health verification

#### Issues Encountered
- None

#### Post-Maintenance Status
- All services running normally
- Performance metrics within normal range
- No user impact

#### Next Maintenance
- Scheduled: 2024-01-22
- Type: Weekly Maintenance
- Estimated Duration: 2 hours
```

#### 4.2 Maintenance Reports
```bash
#!/bin/bash
# Generate maintenance report

echo "=== Maintenance Report ==="
echo "Date: $(date)"
echo "System: AI CV Evaluator"
echo ""

echo "=== System Health ==="
curl -s http://localhost:8080/healthz
echo ""

echo "=== Resource Usage ==="
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"
echo ""

echo "=== Database Status ==="
docker exec ai-cv-evaluator-db psql -U postgres -c "SELECT version();"
echo ""

echo "=== Queue Status ==="
rpk cluster health
echo ""

echo "=== Maintenance Tasks ==="
echo "Completed: $(date)"
echo "Duration: 2 hours"
echo "Status: Success"
```

## Maintenance Tools

### 1. Monitoring Tools

#### 1.1 System Monitoring
```bash
# System resource monitoring
htop
iotop
nethogs

# Docker monitoring
docker stats
docker system df
docker system events

# Database monitoring
pg_top
pg_stat_statements
```

#### 1.2 Application Monitoring
```bash
# Application metrics
curl -s http://localhost:9090/api/v1/query?query=up
curl -s http://localhost:9090/api/v1/query?query=rate(http_requests_total[5m])

# Queue monitoring
rpk cluster health
rpk topic describe evaluate-jobs
rpk group describe ai-cv-evaluator-workers
```

### 2. Maintenance Scripts

#### 2.1 Database Maintenance
```bash
#!/bin/bash
# Database maintenance script

echo "=== Database Maintenance ==="

# Check database health
echo "Checking database health..."
docker exec ai-cv-evaluator-db pg_isready -U postgres

# Run vacuum and analyze
echo "Running vacuum and analyze..."
docker exec ai-cv-evaluator-db psql -U postgres -c "VACUUM ANALYZE;"

# Check for long-running queries
echo "Checking for long-running queries..."
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT pid, now() - query_start AS duration, query 
FROM pg_stat_activity 
WHERE (now() - query_start) > interval '5 minutes';"

# Check database size
echo "Checking database size..."
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT pg_size_pretty(pg_database_size('app')) AS database_size;"

echo "Database maintenance completed"
```

#### 2.2 Log Cleanup
```bash
#!/bin/bash
# Log cleanup script

echo "=== Log Cleanup ==="

# Clean up application logs
echo "Cleaning application logs..."
find /var/log -name "*.log" -mtime +30 -delete
find /var/log -name "*.gz" -mtime +90 -delete

# Clean up Docker logs
echo "Cleaning Docker logs..."
docker system prune -f
docker volume prune -f

# Clean up old backups
echo "Cleaning old backups..."
find /opt/backups -name "*.sql" -mtime +30 -delete
find /opt/backups -name "*.tar.gz" -mtime +90 -delete

echo "Log cleanup completed"
```

#### 2.3 Security Updates
```bash
#!/bin/bash
# Security updates script

echo "=== Security Updates ==="

# Update system packages
echo "Updating system packages..."
apt update && apt upgrade -y

# Check for security vulnerabilities
echo "Checking for vulnerabilities..."
make vuln

# Update Docker images
echo "Updating Docker images..."
docker compose pull

# Check for outdated dependencies
echo "Checking dependencies..."
go list -u -m all
npm outdated

echo "Security updates completed"
```

### 3. Maintenance Automation

#### 3.1 Automated Maintenance
```yaml
# GitHub Actions maintenance workflow
name: Automated Maintenance
on:
  schedule:
    - cron: '0 2 * * 0'  # Weekly on Sunday at 2 AM
  workflow_dispatch:

jobs:
  maintenance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run maintenance
        run: |
          ./scripts/weekly-maintenance.sh
      - name: Generate report
        run: |
          ./scripts/generate-maintenance-report.sh
      - name: Upload report
        uses: actions/upload-artifact@v3
        with:
          name: maintenance-report
          path: maintenance-report.txt
```

#### 3.2 Maintenance Monitoring
```bash
#!/bin/bash
# Maintenance monitoring script

echo "=== Maintenance Monitoring ==="

# Check maintenance mode
echo "Checking maintenance mode..."
curl -s http://localhost:8080/admin/maintenance/status

# Check system health
echo "Checking system health..."
curl -f http://localhost:8080/healthz
curl -f http://localhost:8080/readyz

# Check resource usage
echo "Checking resource usage..."
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# Check queue status
echo "Checking queue status..."
rpk cluster health

echo "Maintenance monitoring completed"
```

## Maintenance Best Practices

### 1. Maintenance Planning

#### 1.1 Maintenance Windows
- **Daily**: 2:00 AM - 2:30 AM (Low traffic)
- **Weekly**: Sunday 3:00 AM - 5:00 AM (Low traffic)
- **Monthly**: First Sunday 4:00 AM - 6:00 AM (Low traffic)
- **Quarterly**: First Sunday 6:00 AM - 10:00 AM (Extended maintenance)

#### 1.2 Maintenance Communication
```markdown
## Maintenance Notification Template

### Subject: Scheduled Maintenance - [Date/Time]

Dear Users,

We will be performing scheduled maintenance on our AI CV Evaluator system.

**Maintenance Window**: [Date] from [Start Time] to [End Time] UTC
**Expected Duration**: [Duration]
**Impact**: [Description of impact]

**What to Expect**:
- Service may be temporarily unavailable
- Some features may be limited
- Performance may be slower than usual

**What We're Doing**:
- [List of maintenance tasks]

We apologize for any inconvenience and appreciate your patience.

Best regards,
The AI CV Evaluator Team
```

### 2. Maintenance Procedures

#### 2.1 Pre-Maintenance
1. **Notify users** 24 hours in advance
2. **Backup system** before maintenance
3. **Verify maintenance scripts** are up to date
4. **Check system health** before starting
5. **Prepare rollback procedures**

#### 2.2 During Maintenance
1. **Set maintenance mode** to prevent new requests
2. **Run maintenance tasks** in sequence
3. **Monitor system health** throughout
4. **Test critical functionality** after each task
5. **Document any issues** encountered

#### 2.3 Post-Maintenance
1. **Verify system health** and functionality
2. **Monitor system performance** for 1 hour
3. **Clear maintenance mode** when ready
4. **Notify users** of completion
5. **Update documentation** with results

### 3. Maintenance Quality Assurance

#### 3.1 Maintenance Testing
```bash
#!/bin/bash
# Maintenance testing script

echo "=== Maintenance Testing ==="

# Test system health
echo "Testing system health..."
curl -f http://localhost:8080/healthz || exit 1
curl -f http://localhost:8080/readyz || exit 1

# Test critical functionality
echo "Testing critical functionality..."
# Test upload
curl -X POST http://localhost:8080/v1/upload \
  -F "cv=@test.txt" \
  -F "project=@test.txt" || exit 1

# Test evaluate
curl -X POST http://localhost:8080/v1/evaluate \
  -H "Content-Type: application/json" \
  -d '{"cv_id":"test","project_id":"test"}' || exit 1

# Test result
curl -f http://localhost:8080/v1/result/test || exit 1

echo "All tests passed"
```

#### 3.2 Maintenance Validation
```bash
#!/bin/bash
# Maintenance validation script

echo "=== Maintenance Validation ==="

# Validate database
echo "Validating database..."
docker exec ai-cv-evaluator-db psql -U postgres -c "SELECT 1;" || exit 1

# Validate queue
echo "Validating queue..."
rpk cluster health | grep -q "Healthy: true" || exit 1

# Validate workers
echo "Validating workers..."
docker ps | grep worker | wc -l

# Validate monitoring
echo "Validating monitoring..."
curl -s http://localhost:9090/api/v1/query?query=up | grep -q "1" || exit 1

echo "Validation completed successfully"
```

---

*This maintenance procedures guide ensures systematic and reliable maintenance of the AI CV Evaluator system while minimizing downtime and maintaining service quality.*
