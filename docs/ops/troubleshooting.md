# Troubleshooting Guide

This comprehensive troubleshooting guide helps diagnose and resolve common issues with the AI CV Evaluator system.

## Quick Diagnosis

### Health Check Commands

```bash
# Check application health
curl -f http://localhost:8080/healthz

# Check readiness
curl -f http://localhost:8080/readyz

# Check metrics
curl http://localhost:8080/metrics

# Check API documentation
curl http://localhost:8080/openapi.yaml
```

### Service Status

```bash
# Check Docker services
docker-compose ps

# Check service logs
docker-compose logs app
docker-compose logs worker
docker-compose logs db
docker-compose logs redpanda
```

## Common Issues and Solutions

### 1. Application Startup Issues

#### Problem: Application fails to start

**Symptoms**:
- Container exits immediately
- No logs in application
- Health check fails

**Diagnosis**:
```bash
# Check container logs
docker-compose logs app

# Check environment variables
docker-compose exec app env | grep -E "(DB_URL|KAFKA_BROKERS)"

# Check port availability
netstat -tulpn | grep :8080
```

**Solutions**:

1. **Database Connection Issues**:
   ```bash
   # Check database connectivity
   docker-compose exec db pg_isready -U postgres
   
   # Verify database URL
   echo $DB_URL
   # Should be: postgres://postgres:postgres@db:5432/app?sslmode=disable
   ```

2. **Queue System Issues**:
   ```bash
   # Check Redpanda status
   docker-compose exec redpanda rpk cluster health
   
   # Verify Kafka brokers
   echo $KAFKA_BROKERS
   # Should be: redpanda:9092
   ```

3. **Port Conflicts**:
   ```bash
   # Check for port conflicts
   lsof -i :8080
   
   # Kill conflicting processes
   sudo kill -9 <PID>
   ```

#### Problem: Environment variables not loaded

**Symptoms**:
- Configuration errors
- Missing API keys
- Default values used

**Solutions**:
```bash
# Check .env file exists
ls -la .env

# Verify environment loading
docker-compose exec app printenv | grep -E "(DB_URL|KAFKA_BROKERS)"

# Decrypt SOPS files if needed
make decrypt-env
```

### 2. Database Issues

#### Problem: Database connection failed

**Symptoms**:
- "database connection failed" errors
- Migration failures
- Readiness check fails

**Diagnosis**:
```bash
# Check database status
docker-compose exec db pg_isready -U postgres

# Check database logs
docker-compose logs db

# Test connection
docker-compose exec app psql $DB_URL -c "SELECT 1"
```

**Solutions**:

1. **Database Not Ready**:
   ```bash
   # Wait for database to be ready
   docker-compose exec db pg_isready -U postgres -d app
   
   # Check database startup logs
   docker-compose logs db | grep "ready to accept connections"
   ```

2. **Migration Issues**:
   ```bash
   # Run migrations manually
   make migrate
   
   # Check migration status
   docker-compose exec db psql $DB_URL -c "SELECT * FROM goose_db_version;"
   ```

3. **Connection String Issues**:
   ```bash
   # Verify connection string format
   echo $DB_URL
   # Format: postgres://user:password@host:port/database?sslmode=disable
   ```

#### Problem: Database performance issues

**Symptoms**:
- Slow queries
- High CPU usage
- Connection timeouts

**Solutions**:
```bash
# Check database performance
docker-compose exec db psql $DB_URL -c "SELECT * FROM pg_stat_activity;"

# Check slow queries
docker-compose exec db psql $DB_URL -c "SELECT query, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Check database size
docker-compose exec db psql $DB_URL -c "SELECT pg_size_pretty(pg_database_size('app'));"
```

### 3. Queue System Issues

#### Problem: Redpanda/Kafka connection failed

**Symptoms**:
- "broker not available" errors
- Message publishing failures
- Consumer group issues

**Diagnosis**:
```bash
# Check Redpanda health
docker-compose exec redpanda rpk cluster health

# Check topic status
docker-compose exec redpanda rpk topic list

# Check consumer groups
docker-compose exec redpanda rpk group list
```

**Solutions**:

1. **Redpanda Not Ready**:
   ```bash
   # Wait for Redpanda to be ready
   docker-compose exec redpanda rpk cluster health
   
   # Check Redpanda logs
   docker-compose logs redpanda | grep "Started Kafka API server"
   ```

2. **Topic Creation Issues**:
   ```bash
   # Create topic manually
   docker-compose exec redpanda rpk topic create evaluate-jobs --partitions 3 --replicas 1
   
   # Check topic configuration
   docker-compose exec redpanda rpk topic describe evaluate-jobs
   ```

3. **Consumer Group Issues**:
   ```bash
   # Reset consumer group
   docker-compose exec redpanda rpk group delete ai-cv-evaluator-workers
   
   # Check consumer lag
   docker-compose exec redpanda rpk group describe ai-cv-evaluator-workers
   ```

#### Problem: Message processing failures

**Symptoms**:
- Jobs stuck in "processing" state
- Worker errors
- Message duplication

**Solutions**:
```bash
# Check worker logs
docker-compose logs worker

# Check message consumption
docker-compose exec redpanda rpk topic consume evaluate-jobs --group ai-cv-evaluator-workers --print-headers

# Reset consumer offset
docker-compose exec redpanda rpk group delete ai-cv-evaluator-workers
```

### 4. AI Service Issues

#### Problem: AI API failures

**Symptoms**:
- "AI service unavailable" errors
- Evaluation jobs failing
- Rate limit errors

**Diagnosis**:
```bash
# Check API keys
echo $OPENROUTER_API_KEY
echo $OPENAI_API_KEY

# Test API connectivity
curl -H "Authorization: Bearer $OPENROUTER_API_KEY" https://openrouter.ai/api/v1/models

# Check AI service logs
docker-compose logs worker | grep -i "ai\|openrouter\|openai"
```

**Solutions**:

1. **API Key Issues**:
   ```bash
   # Verify API keys are set
   docker-compose exec app printenv | grep -E "(OPENROUTER|OPENAI)_API_KEY"
   
   # Test API keys
   curl -H "Authorization: Bearer $OPENROUTER_API_KEY" https://openrouter.ai/api/v1/models
   ```

2. **Rate Limiting**:
   ```bash
   # Check rate limit headers
   curl -I -H "Authorization: Bearer $OPENROUTER_API_KEY" https://openrouter.ai/api/v1/chat/completions
   
   # Implement backoff in configuration
   export AI_BACKOFF_MAX_ELAPSED_TIME=120s
   ```

3. **Model Availability**:
   ```bash
   # Check available models
   curl -H "Authorization: Bearer $OPENROUTER_API_KEY" https://openrouter.ai/api/v1/models | jq '.data[] | select(.pricing.prompt == 0)'
   
   # Set fallback models
   export CHAT_FALLBACK_MODELS=model1,model2,model3
   ```

#### Problem: RAG/Embeddings issues

**Symptoms**:
- "embeddings failed" errors
- Qdrant connection issues
- Vector search failures

**Solutions**:
```bash
# Check Qdrant status
curl http://localhost:6333/collections

# Test embeddings API
curl -H "Authorization: Bearer $OPENAI_API_KEY" https://api.openai.com/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"input": "test", "model": "text-embedding-3-small"}'

# Check Qdrant collections
curl http://localhost:6333/collections
```

### 5. Frontend Issues

#### Problem: Frontend not loading

**Symptoms**:
- 404 errors on frontend routes
- CORS errors
- Static files not served

**Diagnosis**:
```bash
# Check frontend container
docker-compose ps frontend

# Check frontend logs
docker-compose logs frontend

# Test frontend connectivity
curl http://localhost:3001
```

**Solutions**:

1. **Frontend Container Issues**:
   ```bash
   # Restart frontend container
   docker-compose restart frontend
   
   # Check frontend build
   docker-compose exec frontend npm run build
   ```

2. **CORS Issues**:
   ```bash
   # Check CORS configuration
   echo $CORS_ALLOW_ORIGINS
   
   # Update CORS settings
   export CORS_ALLOW_ORIGINS=http://localhost:3001,https://app.example.com
   ```

3. **Static File Issues**:
   ```bash
   # Check Nginx configuration
   docker-compose exec frontend cat /etc/nginx/nginx.conf
   
   # Test static file serving
   curl http://localhost:3001/index.html
   ```

#### Problem: Frontend development issues

**Symptoms**:
- HMR not working
- Build failures
- TypeScript errors

**Solutions**:
```bash
# Check Node.js version
docker-compose exec frontend node --version

# Install dependencies
docker-compose exec frontend npm install

# Check TypeScript errors
docker-compose exec frontend npm run type-check

# Restart development server
docker-compose restart frontend
```

### 6. File Upload Issues

#### Problem: File upload failures

**Symptoms**:
- "file too large" errors
- "unsupported file type" errors
- Upload timeout

**Diagnosis**:
```bash
# Check upload limits
echo $MAX_UPLOAD_MB

# Check file type validation
curl -X POST http://localhost:8080/v1/upload \
  -F "cv=@test.txt" \
  -F "project=@test.txt"

# Check Tika service
curl http://localhost:9998/version
```

**Solutions**:

1. **File Size Issues**:
   ```bash
   # Increase upload limit
   export MAX_UPLOAD_MB=50
   
   # Check file size
   ls -lh test-file.pdf
   ```

2. **File Type Issues**:
   ```bash
   # Check MIME type
   file --mime-type test-file.pdf
   
   # Verify Tika service
   curl http://localhost:9998/version
   ```

3. **Tika Service Issues**:
   ```bash
   # Restart Tika service
   docker-compose restart tika
   
   # Check Tika logs
   docker-compose logs tika
   ```

### 7. Performance Issues

#### Problem: Slow response times

**Symptoms**:
- High latency
- Timeout errors
- Resource exhaustion

**Diagnosis**:
```bash
# Check system resources
docker stats

# Check application metrics
curl http://localhost:8080/metrics

# Check database performance
docker-compose exec db psql $DB_URL -c "SELECT * FROM pg_stat_activity;"
```

**Solutions**:

1. **Database Performance**:
   ```bash
   # Check slow queries
   docker-compose exec db psql $DB_URL -c "SELECT query, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"
   
   # Optimize database
   docker-compose exec db psql $DB_URL -c "VACUUM ANALYZE;"
   ```

2. **Queue Performance**:
   ```bash
   # Check queue metrics
   curl http://localhost:8090/api/topics
   
   # Check consumer lag
   docker-compose exec redpanda rpk group describe ai-cv-evaluator-workers
   ```

3. **AI Service Performance**:
   ```bash
   # Check AI service metrics
   curl http://localhost:8080/metrics | grep ai_
   
   # Optimize AI configuration
   export AI_BACKOFF_MAX_ELAPSED_TIME=60s
   export AI_BACKOFF_INITIAL_INTERVAL=500ms
   ```

### 8. Security Issues

#### Problem: Authentication failures

**Symptoms**:
- Login failures
- Session expired errors
- Unauthorized access

**Solutions**:
```bash
# Check session configuration
echo $ADMIN_SESSION_SECRET

# Test authentication
curl -X POST http://localhost:8080/admin/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=admin&password=changeme"

# Check session cookies
curl -c cookies.txt -X POST http://localhost:8080/admin/login \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "username=admin&password=changeme"
```

#### Problem: CORS issues

**Symptoms**:
- CORS errors in browser
- API calls blocked
- Preflight request failures

**Solutions**:
```bash
# Check CORS configuration
echo $CORS_ALLOW_ORIGINS

# Test CORS headers
curl -H "Origin: http://localhost:3001" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -X OPTIONS http://localhost:8080/v1/upload

# Update CORS settings
export CORS_ALLOW_ORIGINS=http://localhost:3001,https://app.example.com
```

## Monitoring and Debugging

### Log Analysis

#### Application Logs
```bash
# View application logs
docker-compose logs -f app

# Filter error logs
docker-compose logs app | grep -i error

# Filter specific components
docker-compose logs app | grep -i "upload\|evaluate\|result"
```

#### Worker Logs
```bash
# View worker logs
docker-compose logs -f worker

# Filter AI-related logs
docker-compose logs worker | grep -i "ai\|openrouter\|openai"

# Filter job processing logs
docker-compose logs worker | grep -i "job\|evaluate"
```

#### Database Logs
```bash
# View database logs
docker-compose logs -f db

# Filter connection logs
docker-compose logs db | grep -i "connection\|authentication"
```

### Metrics Analysis

#### Application Metrics
```bash
# Get Prometheus metrics
curl http://localhost:8080/metrics

# Filter specific metrics
curl http://localhost:8080/metrics | grep -E "(http_requests_total|jobs_)"

# Check metrics in Prometheus
curl http://localhost:9090/api/v1/query?query=up
```

#### Queue Metrics
```bash
# Check Redpanda Console
open http://localhost:8090

# Check topic metrics
docker-compose exec redpanda rpk topic describe evaluate-jobs

# Check consumer group metrics
docker-compose exec redpanda rpk group describe ai-cv-evaluator-workers
```

### Performance Profiling

#### Database Profiling
```bash
# Check database performance
docker-compose exec db psql $DB_URL -c "SELECT * FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;"

# Check database size
docker-compose exec db psql $DB_URL -c "SELECT pg_size_pretty(pg_database_size('app'));"

# Check table sizes
docker-compose exec db psql $DB_URL -c "SELECT schemaname,tablename,pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size FROM pg_tables WHERE schemaname='public' ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;"
```

#### Application Profiling
```bash
# Check memory usage
docker stats --no-stream

# Check CPU usage
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}"

# Check network usage
docker stats --no-stream --format "table {{.Container}}\t{{.NetIO}}"
```

## Emergency Procedures

### Service Recovery

#### Complete System Restart
```bash
# Stop all services
docker-compose down

# Remove volumes (CAUTION: This will delete data)
docker-compose down -v

# Restart services
docker-compose up -d

# Wait for services to be ready
sleep 30

# Check service health
curl -f http://localhost:8080/healthz
```

#### Database Recovery
```bash
# Backup database
docker-compose exec db pg_dump -U postgres app > backup.sql

# Restore database
docker-compose exec -T db psql -U postgres app < backup.sql

# Check database integrity
docker-compose exec db psql $DB_URL -c "SELECT COUNT(*) FROM uploads;"
```

#### Queue Recovery
```bash
# Reset consumer group
docker-compose exec redpanda rpk group delete ai-cv-evaluator-workers

# Check topic status
docker-compose exec redpanda rpk topic describe evaluate-jobs

# Restart workers
docker-compose restart worker
```

### Data Recovery

#### File Recovery
```bash
# Check uploaded files
docker-compose exec db psql $DB_URL -c "SELECT id, filename, size FROM uploads ORDER BY created_at DESC LIMIT 10;"

# Check job status
docker-compose exec db psql $DB_URL -c "SELECT id, status, created_at FROM jobs ORDER BY created_at DESC LIMIT 10;"

# Check results
docker-compose exec db psql $DB_URL -c "SELECT job_id, cv_match_rate, project_score FROM results ORDER BY created_at DESC LIMIT 10;"
```

#### Log Recovery
```bash
# Save logs to file
docker-compose logs app > app-logs.txt
docker-compose logs worker > worker-logs.txt
docker-compose logs db > db-logs.txt

# Archive logs
tar -czf logs-$(date +%Y%m%d-%H%M%S).tar.gz *.txt
```

## Prevention and Best Practices

### Monitoring Setup

#### Health Checks
```bash
# Set up health check monitoring
curl -f http://localhost:8080/healthz || alert "Application health check failed"
curl -f http://localhost:8080/readyz || alert "Application readiness check failed"
```

#### Resource Monitoring
```bash
# Monitor disk space
df -h | grep -E "(/var/lib/docker|/tmp)"

# Monitor memory usage
free -h

# Monitor CPU usage
top -bn1 | grep "Cpu(s)"
```

### Backup Procedures

#### Database Backup
```bash
# Daily backup script
#!/bin/bash
DATE=$(date +%Y%m%d-%H%M%S)
docker-compose exec db pg_dump -U postgres app > backup-$DATE.sql
gzip backup-$DATE.sql
```

#### Configuration Backup
```bash
# Backup configuration
tar -czf config-backup-$(date +%Y%m%d-%H%M%S).tar.gz .env docker-compose.yml deploy/
```

### Maintenance Procedures

#### Regular Maintenance
```bash
# Weekly maintenance
docker system prune -f
docker volume prune -f

# Database maintenance
docker-compose exec db psql $DB_URL -c "VACUUM ANALYZE;"

# Log rotation
find logs/ -name "*.log" -mtime +7 -delete
```

#### Update Procedures
```bash
# Update application
git pull origin main
docker-compose build
docker-compose up -d

# Update dependencies
docker-compose exec app go mod tidy
docker-compose exec frontend npm update
```

## Getting Help

### Log Collection
```bash
# Collect diagnostic information
mkdir -p diagnostics/$(date +%Y%m%d-%H%M%S)
cd diagnostics/$(date +%Y%m%d-%H%M%S)

# System information
uname -a > system-info.txt
docker version >> system-info.txt
docker-compose version >> system-info.txt

# Service status
docker-compose ps > service-status.txt

# Logs
docker-compose logs app > app-logs.txt
docker-compose logs worker > worker-logs.txt
docker-compose logs db > db-logs.txt
docker-compose logs redpanda > redpanda-logs.txt

# Configuration
cp ../.env env-config.txt
cp ../docker-compose.yml docker-compose-config.txt

# Metrics
curl http://localhost:8080/metrics > metrics.txt

# Archive
cd ..
tar -czf diagnostics-$(date +%Y%m%d-%H%M%S).tar.gz diagnostics/
```

### Support Information

When seeking help, provide:
1. System information (`uname -a`, `docker version`)
2. Service status (`docker-compose ps`)
3. Relevant logs (application, worker, database)
4. Configuration files (`.env`, `docker-compose.yml`)
5. Error messages and stack traces
6. Steps to reproduce the issue

---

*This troubleshooting guide should be updated as new issues are discovered and resolved.*
