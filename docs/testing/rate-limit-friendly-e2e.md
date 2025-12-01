# Rate-Limit-Friendly E2E Testing Strategy

This document describes the rate-limit-friendly E2E testing algorithm designed to run multiple times consecutively without hitting provider rate limits.

## Overview

The AI CV Evaluator uses free-tier LLM providers (Groq and OpenRouter) which have strict rate limits:
- **Groq**: ~30 RPM (requests per minute), ~6K TPM (tokens per minute)
- **OpenRouter**: Variable by model, typically ~20-60 RPM

The standard E2E suite with 14 jobs and ~168K tokens per run would exceed these limits. This rate-limit-friendly strategy uses minimal inputs and proper cooldowns to stay safely under limits.

## Algorithm

### Core Parameters

| Parameter | Default | Purpose |
|-----------|---------|---------|
| `E2E_CORE_JOB_COUNT` | 2 | Number of jobs per test run |
| `E2E_INTER_JOB_COOLDOWN` | 15s | Delay between sequential jobs |
| `E2E_PER_JOB_TIMEOUT` | 120s | Max wait for job completion |
| `E2E_CORE_TIMEOUT` | 8m | Global test timeout |

### Key Design Decisions

1. **Minimal Token Usage**
   - CV texts: ~50-100 characters (vs ~5000 in full tests)
   - Project texts: ~50-100 characters
   - Result: ~200-500 tokens per LLM call instead of thousands

2. **Conservative Request Rate**
   - 2 jobs × 3 LLM calls/job = 6 calls per run
   - 15s cooldown between jobs → ~3 RPM
   - Well under the 30 RPM Groq limit

3. **Multi-Account Fallback**
   - 4 provider accounts (2 Groq + 2 OpenRouter)
   - Automatic rotation and fallback on rate limits
   - Per-account blocking with Retry-After headers

4. **Inter-Run Cooldowns**
   - 30s cooldown between consecutive test runs
   - Allows provider rate windows to reset

## Makefile Targets

### `make test-e2e-core`
Run the core E2E test (requires services to be running):
```bash
make test-e2e-core
```

### `make run-e2e-core`
Run with automatic service startup/cleanup:
```bash
make run-e2e-core
```

### `make run-e2e-core-repeat RUNS=N`
Run multiple times consecutively to validate rate-limit safety:
```bash
make run-e2e-core-repeat RUNS=3
```

### `make test-e2e-single`
Run the fastest possible single-job test:
```bash
make test-e2e-single
```

## CI Integration

For CI pipelines, use the core E2E suite instead of the full suite:

```yaml
# GitHub Actions example
e2e-test:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v4
    - name: Run rate-limit-friendly E2E
      run: make run-e2e-core
      env:
        GROQ_API_KEY: ${{ secrets.GROQ_API_KEY }}
        GROQ_API_KEY_2: ${{ secrets.GROQ_API_KEY_2 }}
        OPENROUTER_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
        OPENROUTER_API_KEY_2: ${{ secrets.OPENROUTER_API_KEY_2 }}
```

## Test Files

- `test/e2e/core_e2e_test.go` - Core rate-limit-friendly tests
  - `TestE2E_Core_RateLimitFriendly` - Main test with configurable jobs
  - `TestE2E_Core_SingleJob` - Single-job quick validation
  - `TestE2E_Core_MultipleRuns` - Simulates consecutive runs

## Validation Results

The algorithm has been validated with consecutive runs:

### Run 1 (Fresh Start)
```
=== Core E2E Test Summary ===
Total jobs: 2
Completed: 2
Successful: 2
Rate-limited: 0
✅ Core E2E test completed successfully
--- PASS: TestE2E_Core_RateLimitFriendly (159.92s)
```

### Run 2 (Immediately After Run 1)
```
=== Core E2E Test Summary ===
Total jobs: 2
Completed: 2
Successful: 1
Rate-limited: 1 (acceptable - test still passes)
✅ Core E2E test completed successfully
--- PASS: TestE2E_Core_RateLimitFriendly (171.93s)
```

**Key Insight**: Even when rate limits are hit on the second run, the test passes because `UPSTREAM_RATE_LIMIT` is treated as an acceptable terminal state. This ensures CI pipelines don't fail due to provider rate limits while still validating that the system can connect to and process requests from real AI providers.

## Configuration Overrides

All parameters can be overridden via environment variables:

```bash
# Run with 3 jobs and 20s cooldown
make run-e2e-core E2E_CORE_JOB_COUNT=3 E2E_INTER_JOB_COOLDOWN=20s

# Run with longer timeout for slow providers
make run-e2e-core E2E_PER_JOB_TIMEOUT=180s E2E_CORE_TIMEOUT=10m
```

## When to Use Full vs Core Suite

| Scenario | Recommended Suite |
|----------|-------------------|
| CI pipeline (frequent runs) | Core (`run-e2e-core`) |
| Pre-release validation | Full (`run-e2e-tests`) |
| Local development | Core or Single |
| Rate limit debugging | Core with `RUNS=3` |

## Troubleshooting

### All jobs hit rate limits
1. Increase `E2E_INTER_JOB_COOLDOWN` to 30s
2. Reduce `E2E_CORE_JOB_COUNT` to 1
3. Wait 60s before retrying

### Jobs stuck in "processing"
1. Increase `E2E_PER_JOB_TIMEOUT` to 180s
2. Check worker logs for LLM response times
3. This is acceptable for slow LLM responses

### Test timeout exceeded
1. Increase `E2E_CORE_TIMEOUT` to 10m
2. Reduce job count
3. Check if services started correctly
