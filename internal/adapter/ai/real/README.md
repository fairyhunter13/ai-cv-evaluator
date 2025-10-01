# AI Real Adapter - Backoff Configuration

This document explains the backoff configuration system implemented for the AI real adapter to address the issue of unit tests using production-grade retry logic with 90-second timeouts.

## Problem

The original implementation used hardcoded backoff configuration with 90-second timeouts, which was inappropriate for fast unit testing. This caused tests to run slowly and timeout in CI/CD environments.

## Solution

### 1. Environment-Based Configuration

The system now uses environment variables to configure backoff behavior:

- **Test Environment** (`APP_ENV=test`): Uses fast timeouts (5s max elapsed time)
- **Production Environment**: Uses configurable timeouts via environment variables

### 2. Configuration Options

#### Environment Variables

```bash
# AI Backoff Configuration
AI_BACKOFF_MAX_ELAPSED_TIME=90s      # Maximum total time for retries
AI_BACKOFF_INITIAL_INTERVAL=1s        # Initial delay between retries
AI_BACKOFF_MAX_INTERVAL=10s           # Maximum delay between retries
AI_BACKOFF_MULTIPLIER=2.0             # Exponential backoff multiplier
```

#### Test Environment Defaults

When `APP_ENV=test`, the system automatically uses:
- Max Elapsed Time: 5 seconds
- Initial Interval: 100ms
- Max Interval: 1 second
- Multiplier: 2.0

### 3. Usage Examples

#### Basic Usage

```go
// Production client with default configuration
cfg := config.Load()
client := real.New(cfg)

// Test client with fast timeouts
testClient := real.NewTestClient(cfg)
```

#### Custom Test Configuration

```go
// Custom backoff for specific test scenarios
customBackoff := &backoff.ExponentialBackOff{
    InitialInterval:     10 * time.Millisecond,
    MaxInterval:          100 * time.Millisecond,
    MaxElapsedTime:       1 * time.Second,
    Multiplier:           1.2,
}

testClient := real.NewTestClientWithCustomBackoff(cfg, customBackoff)
```

#### Environment-Specific Configuration

```go
// Test environment - automatically uses fast timeouts
cfg := config.Config{AppEnv: "test"}
client := real.NewTestClient(cfg)

// Production environment - uses configured values
cfg := config.Config{
    AppEnv: "prod",
    AIBackoffMaxElapsedTime: 120 * time.Second,
    AIBackoffInitialInterval: 2 * time.Second,
    AIBackoffMaxInterval: 15 * time.Second,
    AIBackoffMultiplier: 1.5,
}
client := real.New(cfg)
```

### 4. Implementation Details

#### Config Structure

The `config.Config` struct now includes:

```go
type Config struct {
    // ... existing fields ...
    
    // AI Backoff Configuration
    AIBackoffMaxElapsedTime time.Duration `env:"AI_BACKOFF_MAX_ELAPSED_TIME" envDefault:"90s"`
    AIBackoffInitialInterval time.Duration `env:"AI_BACKOFF_INITIAL_INTERVAL" envDefault:"1s"`
    AIBackoffMaxInterval   time.Duration `env:"AI_BACKOFF_MAX_INTERVAL" envDefault:"10s"`
    AIBackoffMultiplier    float64       `env:"AI_BACKOFF_MULTIPLIER" envDefault:"2.0"`
}
```

#### Helper Methods

```go
// Get environment-appropriate backoff configuration
func (c Config) GetAIBackoffConfig() (maxElapsedTime, initialInterval, maxInterval time.Duration, multiplier float64)

// Check environment type
func (c Config) IsTest() bool
func (c Config) IsDev() bool  
func (c Config) IsProd() bool
```

#### Client Implementation

```go
// Get backoff configuration based on environment
func (c *Client) getBackoffConfig() *backoff.ExponentialBackOff {
    expo := backoff.NewExponentialBackOff()
    
    maxElapsedTime, initialInterval, maxInterval, multiplier := c.cfg.GetAIBackoffConfig()
    expo.MaxElapsedTime = maxElapsedTime
    expo.InitialInterval = initialInterval
    expo.MaxInterval = maxInterval
    expo.Multiplier = multiplier
    
    return expo
}
```

### 5. Test Helpers

#### NewTestClient

Creates a client with test environment configuration:

```go
func NewTestClient(cfg config.Config) *Client {
    cfg.AppEnv = "test"
    return New(cfg)
}
```

#### TestClient with Custom Backoff

For testing specific backoff scenarios:

```go
type TestClient struct {
    *Client
    customBackoff *backoff.ExponentialBackOff
}

func NewTestClientWithCustomBackoff(cfg config.Config, customBackoff *backoff.ExponentialBackOff) *TestClient
```

### 6. Benefits

1. **Fast Unit Tests**: Test environment uses 5-second timeouts instead of 90 seconds
2. **Configurable Production**: Production timeouts can be adjusted via environment variables
3. **Backward Compatible**: Existing code continues to work without changes
4. **Test Flexibility**: Custom backoff configurations for specific test scenarios
5. **Environment Awareness**: Automatic configuration based on environment

### 7. Migration Guide

#### For Existing Tests

Replace:
```go
client := real.New(cfg)
```

With:
```go
client := real.NewTestClient(cfg)
```

#### For Production Configuration

Set environment variables:
```bash
export APP_ENV=prod
export AI_BACKOFF_MAX_ELAPSED_TIME=120s
export AI_BACKOFF_INITIAL_INTERVAL=2s
export AI_BACKOFF_MAX_INTERVAL=15s
export AI_BACKOFF_MULTIPLIER=1.5
```

### 8. Testing

The implementation includes comprehensive tests:

- `TestBackoffConfiguration_TestEnvironment`: Verifies test environment uses fast timeouts
- `TestBackoffConfiguration_ProductionEnvironment`: Verifies production uses configured values
- `TestTestClientWithCustomBackoff`: Tests custom backoff configuration

Run tests with:
```bash
go test ./internal/adapter/ai/real/... -v
```

This solution provides a robust, configurable, and test-friendly backoff system that addresses the original problem while maintaining flexibility for different environments and use cases.
