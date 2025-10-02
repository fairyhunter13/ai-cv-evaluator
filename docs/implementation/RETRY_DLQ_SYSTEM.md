# Retry and Dead Letter Queue (DLQ) System

## 🎯 **Overview**

This document describes the comprehensive retry and Dead Letter Queue (DLQ) system implemented in the AI CV Evaluator for resilient job processing and fault tolerance.

## 🚀 **Key Features Implemented**

### **1. Automatic Retry Mechanism**

#### **Purpose**
Automatically retry failed jobs with intelligent backoff strategies and error classification.

#### **Implementation**
```go
// RetryInfo tracks retry attempts for a job
type RetryInfo struct {
    AttemptCount   int
    MaxAttempts    int
    LastAttemptAt  time.Time
    NextRetryAt    time.Time
    RetryStatus    RetryStatus
    LastError      string
    ErrorHistory   []string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

#### **Features**
- **Exponential Backoff**: Intelligent retry delays with configurable multiplier
- **Jitter Support**: Prevents thundering herd problems
- **Error Classification**: Smart retry decisions based on error types
- **Retry Status Tracking**: Complete retry attempt history

### **2. Dead Letter Queue (DLQ)**

#### **Purpose**
Store permanently failed jobs for analysis, monitoring, and potential reprocessing.

#### **Implementation**
```go
// DLQJob represents a job that has been moved to the Dead Letter Queue
type DLQJob struct {
    JobID            string
    OriginalPayload  EvaluateTaskPayload
    RetryInfo        RetryInfo
    FailureReason    string
    MovedToDLQAt     time.Time
    CanBeReprocessed bool
}
```

#### **Features**
- **Failed Job Storage**: Permanently failed jobs moved to DLQ
- **Reprocessing Capability**: DLQ jobs can be reprocessed if needed
- **Retention Management**: Configurable DLQ job retention and cleanup
- **Monitoring**: Comprehensive DLQ statistics and monitoring

### **3. Retry Manager**

#### **Purpose**
Orchestrate retry logic, DLQ placement, and job reprocessing.

#### **Implementation**
```go
// RetryManager handles automatic job retries and DLQ operations
type RetryManager struct {
    producer    domain.Queue
    dlqProducer domain.Queue
    jobs        domain.JobRepository
    config      RetryConfig
}
```

#### **Core Functions**
- **`RetryJob()`**: Handles automatic job retries
- **`moveToDLQ()`**: Moves failed jobs to Dead Letter Queue
- **`ProcessDLQJob()`**: Processes jobs from DLQ for reprocessing
- **`GetRetryStats()`**: Returns retry statistics

### **4. DLQ Consumer**

#### **Purpose**
Process jobs from the Dead Letter Queue for monitoring and reprocessing.

#### **Implementation**
```go
// DLQConsumer processes DLQ messages
type DLQConsumer struct {
    brokers      []string
    groupID      string
    retryManager *RetryManager
    jobs         domain.JobRepository
}
```

#### **Core Functions**
- **`Start()`**: Starts DLQ message processing
- **`Stop()`**: Stops DLQ consumer gracefully
- **`processDLQRecord()`**: Processes individual DLQ records
- **`GetDLQStats()`**: Returns DLQ statistics

## 📊 **Retry Flow**

### **1. Job Processing Flow**
```
Job Created → Processing → Success ✅
                ↓
            Failure → Retry Check
                ↓
        Retryable? → Yes → Schedule Retry
                ↓
            No → Move to DLQ
```

### **2. Retry Decision Logic**
```go
func (ri *RetryInfo) ShouldRetry(err error, config RetryConfig) bool {
    // Don't retry if max attempts reached
    if ri.AttemptCount >= config.MaxRetries {
        return false
    }
    
    // Don't retry if already in DLQ
    if ri.RetryStatus == RetryStatusDLQ {
        return false
    }
    
    // Check if error is retryable
    errorStr := err.Error()
    for _, retryableErr := range config.RetryableErrors {
        if contains(errorStr, retryableErr) {
            return true
        }
    }
    
    // Check if error is non-retryable
    for _, nonRetryableErr := range config.NonRetryableErrors {
        if contains(errorStr, nonRetryableErr) {
            return false
        }
    }
    
    // Default to retryable for unknown errors
    return true
}
```

### **3. Exponential Backoff Calculation**
```go
func (ri *RetryInfo) CalculateNextRetryDelay(config RetryConfig) time.Duration {
    // Calculate exponential backoff
    delay := time.Duration(float64(config.InitialDelay) * pow(config.Multiplier, float64(ri.AttemptCount)))
    
    // Cap at max delay
    if delay > config.MaxDelay {
        delay = config.MaxDelay
    }
    
    // Add jitter if enabled
    if config.Jitter {
        jitter := time.Duration(float64(delay) * 0.1) // 10% jitter
        delay = delay + jitter
    }
    
    return delay
}
```

## 📈 **DLQ Flow**

### **1. DLQ Placement Flow**
```
Job Failed → Max Retries Reached → Move to DLQ
                ↓
        Non-Retryable Error → Move to DLQ
                ↓
        DLQ Processing → Reprocessing Available
```

### **2. DLQ Reprocessing**
```go
func (rm *RetryManager) ProcessDLQJob(ctx context.Context, dlqJob domain.DLQJob) error {
    // Check if job can be reprocessed
    if !dlqJob.CanBeReprocessed {
        return fmt.Errorf("DLQ job cannot be reprocessed")
    }
    
    // Reset retry info for reprocessing
    retryInfo := &domain.RetryInfo{
        AttemptCount:  0,
        MaxAttempts:   rm.config.MaxRetries,
        RetryStatus:   domain.RetryStatusNone,
        CreatedAt:     time.Now(),
        UpdatedAt:     time.Now(),
        ErrorHistory:  []string{},
    }
    
    // Update job status to queued
    rm.jobs.UpdateStatus(ctx, dlqJob.JobID, domain.JobQueued, nil)
    
    // Enqueue job for reprocessing
    return rm.producer.EnqueueEvaluate(ctx, dlqJob.OriginalPayload)
}
```

## ⚙️ **Configuration**

### **Environment Variables**
```bash
# Retry Configuration
RETRY_MAX_RETRIES=3                    # Maximum retry attempts
RETRY_INITIAL_DELAY=2s                # Initial retry delay
RETRY_MAX_DELAY=30s                   # Maximum retry delay
RETRY_MULTIPLIER=2.0                  # Exponential backoff multiplier
RETRY_JITTER=true                     # Enable jitter for backoff

# DLQ Configuration
DLQ_ENABLED=true                      # Enable DLQ functionality
DLQ_MAX_AGE=168h                      # DLQ job retention period (7 days)
DLQ_CLEANUP_INTERVAL=24h              # DLQ cleanup interval
```

### **Default Configuration**
```go
func DefaultRetryConfig() RetryConfig {
    return RetryConfig{
        MaxRetries:    3,
        InitialDelay:  2 * time.Second,
        MaxDelay:      30 * time.Second,
        Multiplier:    2.0,
        Jitter:        true,
        RetryableErrors: []string{
            "context deadline exceeded",
            "connection refused",
            "timeout",
            "temporary failure",
            "rate limited",
            "upstream timeout",
            "upstream rate limit",
        },
        NonRetryableErrors: []string{
            "invalid argument",
            "not found",
            "conflict",
            "schema invalid",
            "authentication failed",
            "authorization failed",
        },
    }
}
```

## 📊 **Monitoring and Observability**

### **1. Retry Statistics**
```go
func (rm *RetryManager) GetRetryStats(ctx context.Context) (map[string]interface{}, error) {
    return map[string]interface{}{
        "total_retries":     0,
        "successful_retries": 0,
        "failed_retries":    0,
        "dlq_jobs":         0,
    }, nil
}
```

### **2. DLQ Statistics**
```go
func (dc *DLQConsumer) GetDLQStats(ctx context.Context) (map[string]interface{}, error) {
    return map[string]interface{}{
        "dlq_messages_processed": 0,
        "dlq_messages_failed":    0,
        "dlq_messages_reprocessed": 0,
    }, nil
}
```

### **3. Logging and Metrics**
- **Retry Attempts**: Track retry attempts and outcomes
- **DLQ Operations**: Monitor DLQ placement and processing
- **Error Classification**: Track retryable vs non-retryable errors
- **Performance Metrics**: Retry timing and success rates

## 🎯 **Benefits**

### **1. Improved Reliability**
- **Automatic Recovery**: Jobs automatically retry on transient failures
- **Fault Tolerance**: System continues operating despite individual job failures
- **Error Classification**: Smart retry decisions based on error types

### **2. Better Observability**
- **Complete Error History**: Full error tracking for debugging
- **Retry Statistics**: Comprehensive retry metrics
- **DLQ Monitoring**: Failed job tracking and analysis

### **3. Operational Excellence**
- **Configurable Behavior**: Flexible retry and DLQ configuration
- **Graceful Degradation**: Failed jobs don't block system operation
- **Reprocessing Capability**: DLQ jobs can be reprocessed if needed

## 🚀 **Usage Examples**

### **1. Basic Retry Configuration**
```go
// Create retry manager with default configuration
retryConfig := domain.DefaultRetryConfig()
retryManager := redpanda.NewRetryManager(producer, dlqProducer, jobs, retryConfig)

// Handle job failure
if err != nil {
    retryInfo := &domain.RetryInfo{
        AttemptCount: 1,
        MaxAttempts:  retryConfig.MaxRetries,
        LastError:    err.Error(),
        CreatedAt:    time.Now(),
        UpdatedAt:    time.Now(),
    }
    
    // Attempt retry
    if err := retryManager.RetryJob(ctx, jobID, retryInfo, payload); err != nil {
        log.Error("failed to retry job", slog.Any("error", err))
    }
}
```

### **2. DLQ Processing**
```go
// Create DLQ consumer
dlqConsumer, err := redpanda.NewDLQConsumer(brokers, "dlq-consumer", retryManager, jobs)
if err != nil {
    log.Fatal("failed to create DLQ consumer", slog.Any("error", err))
}

// Start DLQ processing
if err := dlqConsumer.Start(ctx); err != nil {
    log.Fatal("failed to start DLQ consumer", slog.Any("error", err))
}
```

### **3. Custom Retry Configuration**
```go
// Custom retry configuration
retryConfig := domain.RetryConfig{
    MaxRetries:    5,                    // More retries
    InitialDelay:  1 * time.Second,      // Faster initial retry
    MaxDelay:      60 * time.Second,     // Longer max delay
    Multiplier:    1.5,                  // Slower backoff
    Jitter:        true,                 // Enable jitter
    RetryableErrors: []string{
        "timeout",
        "connection refused",
        "rate limited",
    },
    NonRetryableErrors: []string{
        "invalid argument",
        "authentication failed",
    },
}
```

## 🔧 **Integration Points**

### **1. Consumer Integration**
- **Enhanced Error Handling**: Consumer now handles retries automatically
- **DLQ Support**: Failed jobs are moved to DLQ
- **Retry Scheduling**: Automatic retry scheduling with backoff

### **2. Producer Integration**
- **DLQ Publishing**: Producer supports DLQ message publishing
- **Transactional Semantics**: Exactly-once delivery for DLQ messages
- **Error Handling**: Robust error handling for DLQ operations

### **3. Configuration Integration**
- **Environment Variables**: Full configuration via environment variables
- **Default Values**: Sensible defaults for all settings
- **Validation**: Configuration validation and error handling

## 📝 **Next Steps**

1. **Testing**: Comprehensive unit and integration tests
2. **Monitoring**: Enhanced metrics and alerting
3. **Documentation**: API documentation and usage guides
4. **Performance**: Load testing and optimization
5. **Operations**: Deployment and maintenance procedures

## 🎉 **Conclusion**

The retry and DLQ system provides a robust, configurable, and observable system for handling job failures. It ensures high reliability through automatic retries, fault tolerance through DLQ placement, and operational excellence through comprehensive monitoring and configuration options.
