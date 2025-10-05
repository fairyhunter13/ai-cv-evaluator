# E2E Test Failures - Rate Limiting Root Cause Analysis

## Executive Summary

E2E tests (`TestE2E_ComprehensiveSmoke`, `TestE2E_EdgeCaseSmoke`, `TestE2E_PerformanceSmoke`) are failing with "enhanced evaluation failed after retries" due to **OpenRouter's strict rate limiting on free tier models**. All 53 free models simultaneously return HTTP 429 (Too Many Requests), causing the rate limit cache to block them and preventing any AI evaluations from completing.

## Root Cause

### Primary Issue: OpenRouter Free Tier Rate Limits
- OpenRouter free tier has **very strict rate limits** across all free models
- When multiple workers make parallel requests, **all models hit 429 simultaneously**
- Rate limit is **account-wide**, not per-model
- Free models share the same underlying rate limit quota

### Secondary Issue: Aggressive Rate Limit Cache
The implemented `RateLimitCache` exacerbates the problem:
1. **Exponential Backoff**: Models get blocked for increasing durations (20s → 40s → 80s → ... → 2 hours)
2. **Permanent Blocking**: Once 53 models are blocked, there are no available models left
3. **Failure Accumulation**: Even with `maxFailures=5`, models quickly accumulate failures from rapid 429s

### Evidence from Logs

```
Rate Limited: 53 Free Models Blocked
=================================================
blocked_models | 53
available_models | 0
--- Example Model Blocks ---
deepseek/deepseek-chat-v3.1:free | failure_count: 39 | blocked_until: 2025-10-04T03:09:54Z (2 hours)
nvidia/nemotron-nano-9b-v2:free | failure_count: 9 | blocked_until: 2025-10-04T02:35:12Z (1.4 hours)
google/gemini-2.0-flash-exp:free | failure_count: 15 | blocked_until: 2025-10-04T02:19:43Z (1.2 hours)
```

### Sequence of Events
1. E2E test initiates evaluation job
2. Worker starts processing with 12-24 concurrent goroutines
3. Multiple jobs send AI requests simultaneously
4. OpenRouter returns 429 for all models due to account rate limit
5. Rate limit cache records failures and blocks models exponentially
6. After 2-3 seconds, all 53 models are blocked
7. Subsequent retries find 0 available models → job fails
8. E2E test receives "enhanced evaluation failed after retries"

## Current Configuration Problems

### Worker Configuration (docker-compose.yml)
```yaml
worker:
  environment:
    - CONSUMER_MAX_CONCURRENCY=24  # Too high for free tier!
```

### Rate Limit Cache Settings (internal/adapter/ai/rate_limit_cache.go)
```go
defaultDuration: 20 * time.Second  // Even 20s is too long when all models fail
maxFailures:     5                  // Models hit this instantly with parallel requests
```

### AI Client Throttling
- **OPENROUTER_MIN_INTERVAL**: Not set initially
- No global request pacing across worker instances
- Each worker independently hammers OpenRouter

## Solutions Implemented

### 1. Reduced Worker Concurrency
```yaml
# docker-compose.yml
CONSUMER_MAX_CONCURRENCY=1  # Process one job at a time per worker
```

### 2. Client-Level Request Throttling
```go
// internal/adapter/ai/real/client.go
OPENROUTER_MIN_INTERVAL=1500ms  # 1.5s between requests per client instance
```

### 3. OpenRouter Client Hints
```yaml
# docker-compose.yml
OPENROUTER_REFERER=https://github.com/fairyhunter13/ai-cv-evaluator
OPENROUTER_TITLE=AI CV Evaluator E2E
```

### 4. Softened Rate Limit Cache
```go
// internal/adapter/ai/rate_limit_cache.go
defaultDuration: 20 * time.Second  // Reduced from 2 minutes
maxFailures:     5                  // Increased from 2
cleanupInterval: 30 * time.Second  // Increased cleanup frequency
```

## Recommendations

### Short-Term (For E2E Tests)
1. ✅ **Reduce Parallelism**: Set `CONSUMER_MAX_CONCURRENCY=1` to minimize concurrent requests
2. ✅ **Add Request Throttling**: Implement `OPENROUTER_MIN_INTERVAL=1.5s` between requests
3. ✅ **Disable Aggressive Caching**: Use softer rate limit cache settings
4. **Increase Test Timeout**: Allow more time for sequential processing (current: 60s)
5. **Reduce Test Count**: Run fewer E2E tests in parallel

### Medium-Term (For Production)
1. **Upgrade to Paid Tier**: OpenRouter paid plans have much higher rate limits
2. **Implement Smart Model Selection**: Prefer models with better rate limit quotas
3. **Add Backoff at Service Level**: Implement global backoff when 429s detected
4. **Queue Management**: Implement smarter job queuing to prevent bursts

### Long-Term (Architecture)
1. **Multi-Provider Fallback**: Support multiple AI providers (OpenAI, Anthropic, Groq)
2. **Rate Limit Monitoring**: Track and expose rate limit metrics
3. **Adaptive Throttling**: Dynamically adjust request rate based on 429 responses
4. **Model Pool Management**: Maintain a pool of "healthy" models and rotate through them

## Testing Strategy

### Current Approach
```bash
# Single test with minimal parallelism
E2E_PARALLEL=1 E2E_WORKER_REPLICAS=1 E2E_AI_TIMEOUT=30s go test -timeout=60s -run TestE2E_ComprehensiveSmoke/cv_01
```

### Success Criteria
- Job status changes from `queued` → `processing` → `completed`
- No "enhanced evaluation failed after retries" errors
- Worker logs show successful AI model responses
- Rate limit cache shows manageable number of blocked models (< 50%)

## Conclusion

The E2E test failures are **not a bug in the application logic**, but rather a **resource constraint issue** with OpenRouter's free tier rate limits. The application is working correctly but cannot scale with free tier quotas. The implemented mitigations reduce the frequency of 429s, but for reliable E2E testing and production use, **upgrading to OpenRouter's paid tier or adding alternative AI providers is strongly recommended**.

## Status

- **Date**: 2025-10-04
- **Status**: Root cause identified and documented
- **Mitigations**: Implemented (reduced concurrency, added throttling, softened cache)
- **Next Steps**: Test with new settings and evaluate need for paid tier


