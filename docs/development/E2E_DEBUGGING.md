# E2E Testing Debugging Guide

This document provides comprehensive guidance on debugging End-to-End (E2E) tests for the AI CV Evaluator project.

## Overview

E2E tests are critical for ensuring the complete system functionality works as expected. This guide covers debugging techniques, common issues, and troubleshooting procedures.

## E2E Test Structure

### 1. Test Organization

```
test/e2e/
├── happy_path_e2e_test.go          # Core workflow tests
├── smoke_random_e2e_test.go        # Random data tests
├── rfc_real_responses_e2e_test.go  # RFC evidence tests
├── helpers_e2e_test.go             # Test utilities
└── helpers_smoke_e2e_test.go       # Smoke test utilities
```

### 2. Test Categories

#### 2.1 Happy Path Tests
- **Purpose**: Test complete workflow from upload to result
- **Coverage**: Core functionality validation
- **Duration**: 90 seconds timeout
- **Data**: Simple test data

#### 2.2 Smoke Tests
- **Purpose**: Random testing with diverse data
- **Coverage**: System stability validation
- **Duration**: 60 seconds timeout
- **Data**: Random test data pairs

#### 2.3 RFC Evidence Tests
- **Purpose**: Generate real responses for documentation
- **Coverage**: Real-world scenario validation
- **Duration**: 120 seconds timeout
- **Data**: Real candidate CV and project report

## Debugging Techniques

### 1. Test Execution Debugging

#### 1.1 Verbose Test Execution
```bash
# Run E2E tests with verbose output
go test -tags=e2e -v -race -failfast -count=1 -timeout=300s ./test/e2e/...

# Run specific test with verbose output
go test -tags=e2e -v -race -failfast -count=1 -timeout=300s -run TestE2E_HappyPath ./test/e2e/...

# Run with detailed logging
E2E_DEBUG=true go test -tags=e2e -v -race -failfast -count=1 -timeout=300s ./test/e2e/...
```

#### 1.2 Test Isolation
```bash
# Run single test in isolation
go test -tags=e2e -v -race -failfast -count=1 -timeout=300s -run TestE2E_HappyPath_UploadEvaluateResult ./test/e2e/

# Run with specific base URL
E2E_BASE_URL="http://localhost:8080/v1" go test -tags=e2e -v -race -failfast -count=1 -timeout=300s ./test/e2e/
```

### 2. Service Health Debugging

#### 2.1 Pre-Test Health Checks
```bash
# Check application health
curl -f http://localhost:8080/healthz
curl -f http://localhost:8080/readyz

# Check database connectivity
docker exec ai-cv-evaluator-db pg_isready -U postgres

# Check queue health
rpk cluster health

# Check worker processes
docker ps | grep worker
```

#### 2.2 Service Status Monitoring
```bash
# Monitor service logs during tests
docker compose logs -f app worker

# Check resource usage
docker stats --no-stream

# Monitor queue metrics
rpk group describe ai-cv-evaluator-workers
```

### 3. Test Data Debugging

#### 3.1 Test Data Validation
```bash
# Check test data availability
ls -la test/testdata/

# Validate test data content
head -5 test/testdata/cv_01.txt
head -5 test/testdata/project_01.txt

# Check test data pairs
go run test/e2e/helpers_e2e_test.go
```

#### 3.2 Test Data Issues
```bash
# Check for empty test files
find test/testdata/ -size 0

# Check for corrupted test files
file test/testdata/*.txt

# Validate test data encoding
iconv -f utf-8 -t utf-8 test/testdata/cv_01.txt > /dev/null
```

### 4. Network Debugging

#### 4.1 HTTP Request Debugging
```bash
# Test API endpoints manually
curl -v http://localhost:8080/healthz
curl -v http://localhost:8080/readyz

# Test upload endpoint
curl -v -X POST http://localhost:8080/v1/upload \
  -F "cv=@test/testdata/cv_01.txt" \
  -F "project=@test/testdata/project_01.txt"

# Test evaluate endpoint
curl -v -X POST http://localhost:8080/v1/evaluate \
  -H "Content-Type: application/json" \
  -d '{"cv_id":"test","project_id":"test"}'
```

#### 4.2 Network Connectivity Issues
```bash
# Check port availability
netstat -tlnp | grep :8080
netstat -tlnp | grep :5432
netstat -tlnp | grep :9092

# Test network connectivity
telnet localhost 8080
telnet localhost 5432
telnet localhost 9092
```

### 5. Database Debugging

#### 5.1 Database Connection Issues
```bash
# Check database status
docker exec ai-cv-evaluator-db psql -U postgres -c "SELECT version();"

# Check database tables
docker exec ai-cv-evaluator-db psql -U postgres -c "\dt"

# Check database connections
docker exec ai-cv-evaluator-db psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"
```

#### 5.2 Database Performance Issues
```bash
# Check slow queries
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT query, calls, total_time, mean_time 
FROM pg_stat_statements 
ORDER BY mean_time DESC 
LIMIT 10;"

# Check database locks
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT * FROM pg_locks WHERE NOT granted;"

# Check database size
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT pg_size_pretty(pg_database_size('app'));"
```

### 6. Queue Debugging

#### 6.1 Queue Health Issues
```bash
# Check queue cluster health
rpk cluster health

# Check topic status
rpk topic describe evaluate-jobs

# Check consumer group status
rpk group describe ai-cv-evaluator-workers

# Check queue lag
rpk group describe ai-cv-evaluator-workers | grep -i lag
```

#### 6.2 Queue Performance Issues
```bash
# Check queue throughput
rpk topic consume evaluate-jobs --num 10 --print-headers

# Check queue metrics
curl -s http://localhost:9090/api/v1/query?query=queue_lag_seconds

# Check worker processing
docker logs ai-cv-evaluator-worker --tail 100
```

### 7. Worker Debugging

#### 7.1 Worker Process Issues
```bash
# Check worker status
docker ps | grep worker

# Check worker logs
docker logs ai-cv-evaluator-worker --tail 100

# Check worker resource usage
docker stats ai-cv-evaluator-worker

# Check worker configuration
docker exec ai-cv-evaluator-worker env | grep -E "(DB_|KAFKA_|AI_)"
```

#### 7.2 Worker Performance Issues
```bash
# Check worker CPU usage
docker exec ai-cv-evaluator-worker top -bn1 | grep "Cpu(s)"

# Check worker memory usage
docker exec ai-cv-evaluator-worker free -h

# Check worker goroutines
docker exec ai-cv-evaluator-worker curl -s http://localhost:8080/debug/pprof/goroutine
```

## Common Issues and Solutions

### 1. Test Timeout Issues

#### 1.1 Service Startup Timeouts
**Problem**: Services not ready within timeout period
**Solution**:
```bash
# Increase service startup timeout
docker compose up -d --timeout 300

# Check service health before tests
./scripts/wait-for-services.sh

# Use health check endpoints
curl -f http://localhost:8080/healthz
```

#### 1.2 Test Execution Timeouts
**Problem**: Tests taking longer than expected
**Solution**:
```bash
# Increase test timeout
go test -tags=e2e -timeout=600s ./test/e2e/...

# Check for resource constraints
docker stats --no-stream

# Optimize test data size
head -100 test/testdata/cv_01.txt > test/testdata/cv_01_small.txt
```

### 2. Service Connectivity Issues

#### 2.1 Port Conflicts
**Problem**: Services unable to bind to ports
**Solution**:
```bash
# Check port usage
netstat -tlnp | grep :8080

# Kill conflicting processes
sudo lsof -ti:8080 | xargs kill -9

# Use different ports
PORT=8081 go test -tags=e2e ./test/e2e/...
```

#### 2.2 Network Configuration Issues
**Problem**: Services unable to communicate
**Solution**:
```bash
# Check Docker network
docker network ls
docker network inspect ai-cv-evaluator_default

# Restart Docker network
docker compose down
docker compose up -d
```

### 3. Database Issues

#### 3.1 Connection Pool Exhaustion
**Problem**: Database connection pool exhausted
**Solution**:
```bash
# Check connection count
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT count(*) FROM pg_stat_activity;"

# Increase connection pool
# Update docker-compose.yml with higher max_connections

# Restart database
docker compose restart db
```

#### 3.2 Database Lock Issues
**Problem**: Database locks preventing operations
**Solution**:
```bash
# Check for locks
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT * FROM pg_locks WHERE NOT granted;"

# Kill blocking queries
docker exec ai-cv-evaluator-db psql -U postgres -c "
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state = 'idle in transaction';"
```

### 4. Queue Issues

#### 4.1 Queue Lag Issues
**Problem**: High queue lag affecting test performance
**Solution**:
```bash
# Check queue lag
rpk group describe ai-cv-evaluator-workers

# Scale workers
docker compose up -d --scale worker=8

# Check queue configuration
rpk topic describe evaluate-jobs
```

#### 4.2 Message Processing Issues
**Problem**: Messages not being processed
**Solution**:
```bash
# Check consumer group status
rpk group describe ai-cv-evaluator-workers

# Reset consumer group offset
rpk group delete ai-cv-evaluator-workers

# Restart workers
docker compose restart worker
```

### 5. AI Processing Issues

#### 5.1 AI API Timeouts
**Problem**: AI requests timing out
**Solution**:
```bash
# Check AI API configuration
docker exec ai-cv-evaluator-worker env | grep -E "(AI_|OPENROUTER_)"

# Test AI API connectivity
curl -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  https://openrouter.ai/api/v1/models

# Increase AI timeout
# Update configuration with higher timeout values
```

#### 5.2 AI Rate Limiting
**Problem**: AI API rate limiting
**Solution**:
```bash
# Check AI API usage
curl -H "Authorization: Bearer $OPENROUTER_API_KEY" \
  https://openrouter.ai/api/v1/auth/key

# Implement rate limiting
# Add delays between AI requests
```

## Debugging Tools

### 1. Log Analysis

#### 1.1 Application Logs
```bash
# View application logs
docker logs ai-cv-evaluator-app --tail 100

# Follow application logs
docker logs ai-cv-evaluator-app -f

# Filter error logs
docker logs ai-cv-evaluator-app 2>&1 | grep ERROR

# Filter specific test logs
docker logs ai-cv-evaluator-app 2>&1 | grep "test-e2e"
```

#### 1.2 Worker Logs
```bash
# View worker logs
docker logs ai-cv-evaluator-worker --tail 100

# Follow worker logs
docker logs ai-cv-evaluator-worker -f

# Filter processing logs
docker logs ai-cv-evaluator-worker 2>&1 | grep "processing"
```

#### 1.3 Database Logs
```bash
# View database logs
docker logs ai-cv-evaluator-db --tail 100

# Follow database logs
docker logs ai-cv-evaluator-db -f

# Filter slow query logs
docker logs ai-cv-evaluator-db 2>&1 | grep "slow"
```

### 2. Performance Monitoring

#### 2.1 Resource Monitoring
```bash
# Monitor system resources
docker stats --no-stream

# Monitor specific containers
docker stats ai-cv-evaluator-app ai-cv-evaluator-worker

# Monitor disk usage
df -h
du -sh /var/lib/docker/volumes/*
```

#### 2.2 Application Metrics
```bash
# Check Prometheus metrics
curl -s http://localhost:9090/api/v1/query?query=up

# Check application metrics
curl -s http://localhost:8080/metrics

# Check queue metrics
curl -s http://localhost:9090/api/v1/query?query=queue_lag_seconds
```

### 3. Network Debugging

#### 3.1 Network Connectivity
```bash
# Test HTTP connectivity
curl -v http://localhost:8080/healthz

# Test database connectivity
telnet localhost 5432

# Test queue connectivity
telnet localhost 9092

# Test internal network
docker exec ai-cv-evaluator-app ping db
docker exec ai-cv-evaluator-app ping redpanda
```

#### 3.2 Network Performance
```bash
# Check network latency
ping localhost

# Check network throughput
iperf3 -s &
iperf3 -c localhost

# Check network errors
netstat -i
```

### 4. Test Data Debugging

#### 4.1 Test Data Validation
```bash
# Validate test data files
find test/testdata/ -type f -exec file {} \;

# Check test data encoding
file test/testdata/*.txt

# Validate test data content
head -5 test/testdata/cv_01.txt
head -5 test/testdata/project_01.txt
```

#### 4.2 Test Data Issues
```bash
# Check for empty files
find test/testdata/ -size 0

# Check for corrupted files
find test/testdata/ -name "*.txt" -exec grep -L ".*" {} \;

# Check file permissions
ls -la test/testdata/
```

## Test Environment Setup

### 1. Development Environment

#### 1.1 Local Development
```bash
# Start development environment
make dev-full

# Check service health
curl -f http://localhost:8080/healthz

# Run E2E tests
make test-e2e
```

#### 1.2 Test Environment
```bash
# Start test environment
docker compose -f docker-compose.test.yml up -d

# Run E2E tests
E2E_BASE_URL="http://localhost:8080/v1" make test-e2e
```

### 2. CI/CD Environment

#### 2.1 GitHub Actions
```yaml
# E2E test workflow
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Start services
        run: docker compose up -d
      - name: Wait for services
        run: ./scripts/wait-for-services.sh
      - name: Run E2E tests
        run: make test-e2e
```

#### 2.2 Local CI Testing
```bash
# Run CI E2E tests
make ci-e2e

# Check CI logs
ls -la artifacts/ci-e2e-logs-*/

# Analyze CI results
cat artifacts/ci-e2e-logs-*/compose.errors.log
```

## Best Practices

### 1. Test Design

#### 1.1 Test Isolation
- Each test should be independent
- Use unique test data for each test
- Clean up test data after each test
- Avoid shared state between tests

#### 1.2 Test Reliability
- Use deterministic test data
- Implement proper retry logic
- Handle flaky network conditions
- Use appropriate timeouts

### 2. Debugging Strategy

#### 2.1 Systematic Approach
1. **Identify the issue**: What is failing?
2. **Check service health**: Are all services running?
3. **Verify test data**: Is test data valid?
4. **Check network connectivity**: Can services communicate?
5. **Review logs**: What errors are occurring?
6. **Test manually**: Can you reproduce the issue?

#### 2.2 Documentation
- Document common issues
- Keep debugging procedures updated
- Share solutions with team
- Learn from each debugging session

### 3. Performance Optimization

#### 3.1 Test Performance
- Use appropriate test data sizes
- Optimize test execution time
- Parallelize tests where possible
- Monitor test performance trends

#### 3.2 System Performance
- Monitor resource usage during tests
- Optimize service configurations
- Scale services appropriately
- Use performance monitoring tools

---

*This E2E debugging guide ensures efficient troubleshooting and resolution of test issues for the AI CV Evaluator project.*
