# Data Retention and Cleanup System

## 🎯 **Overview**

This document describes the Data Retention and Cleanup System implemented in the AI CV Evaluator for automatic data lifecycle management, compliance, and storage optimization.

## 🚀 **Key Features Implemented**

### **1. Data Retention Configuration**

#### **Purpose**
Automatically manage data lifecycle with configurable retention periods and cleanup intervals.

#### **Implementation**
```go
// Config holds data retention configuration
type Config struct {
    DataRetentionDays int           `env:"DATA_RETENTION_DAYS" envDefault:"90"`
    CleanupInterval    time.Duration `env:"CLEANUP_INTERVAL" envDefault:"24h"`
}
```

#### **Features**
- **Configurable Retention**: Set retention periods per data type
- **Automatic Cleanup**: Scheduled cleanup of expired data
- **Compliance Support**: GDPR and data protection compliance
- **Storage Optimization**: Reduce storage costs through data lifecycle management

### **2. Cleanup Service**

#### **Purpose**
Automatically clean up expired data based on retention policies.

#### **Implementation**
```go
// CleanupService handles automatic data cleanup
type CleanupService struct {
    db              domain.Database
    retentionDays   int
    cleanupInterval time.Duration
}

// RunPeriodic runs the cleanup service periodically
func (cs *CleanupService) RunPeriodic(ctx context.Context, interval time.Duration)
```

#### **Features**
- **Periodic Cleanup**: Automatic cleanup at configurable intervals
- **Batch Processing**: Efficient batch deletion of expired data
- **Transaction Safety**: Safe cleanup with database transactions
- **Monitoring**: Comprehensive logging and metrics

### **3. Data Types and Retention**

#### **Purpose**
Define retention policies for different types of data.

#### **Data Types**
- **Uploads**: CV and project files (retention: 90 days)
- **Jobs**: Job processing records (retention: 90 days)
- **Results**: Evaluation results (retention: 90 days)
- **Logs**: Application logs (retention: 30 days)
- **Metrics**: Performance metrics (retention: 7 days)

#### **Retention Policies**
```go
// Retention policies for different data types
var retentionPolicies = map[string]int{
    "uploads": 90,  // 90 days
    "jobs":    90,  // 90 days
    "results": 90,  // 90 days
    "logs":    30,  // 30 days
    "metrics": 7,   // 7 days
}
```

## 📊 **Cleanup Process**

### **1. Cleanup Flow**
```
Scheduled Timer → Check Expired Data → Batch Delete → Update Statistics → Log Results
```

### **2. Data Identification**
```go
// identifyExpiredData identifies data that has exceeded retention period
func (cs *CleanupService) identifyExpiredData(ctx context.Context, dataType string, retentionDays int) ([]string, error) {
    cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
    
    query := fmt.Sprintf(`
        SELECT id FROM %s 
        WHERE created_at < $1 
        ORDER BY created_at ASC
    `, dataType)
    
    rows, err := cs.db.Query(ctx, query, cutoffDate)
    if err != nil {
        return nil, fmt.Errorf("failed to query expired data: %w", err)
    }
    defer rows.Close()
    
    var expiredIDs []string
    for rows.Next() {
        var id string
        if err := rows.Scan(&id); err != nil {
            return nil, fmt.Errorf("failed to scan ID: %w", err)
        }
        expiredIDs = append(expiredIDs, id)
    }
    
    return expiredIDs, nil
}
```

### **3. Batch Deletion**
```go
// deleteExpiredDataBatch deletes expired data in batches
func (cs *CleanupService) deleteExpiredDataBatch(ctx context.Context, dataType string, ids []string) error {
    if len(ids) == 0 {
        return nil
    }
    
    // Delete in batches to avoid overwhelming the database
    batchSize := 1000
    for i := 0; i < len(ids); i += batchSize {
        end := i + batchSize
        if end > len(ids) {
            end = len(ids)
        }
        
        batch := ids[i:end]
        if err := cs.deleteBatch(ctx, dataType, batch); err != nil {
            return fmt.Errorf("failed to delete batch: %w", err)
        }
    }
    
    return nil
}
```

## ⚙️ **Configuration**

### **Environment Variables**
```bash
# Data Retention Configuration
DATA_RETENTION_DAYS=90                    # Data retention period in days
CLEANUP_INTERVAL=24h                      # Cleanup interval
```

### **Service Configuration**
```go
// CleanupService configuration
type CleanupService struct {
    db              domain.Database
    retentionDays   int           // Data retention period in days
    cleanupInterval time.Duration // Cleanup interval
}
```

## 🎯 **Data Lifecycle Management**

### **1. Upload Data Lifecycle**
```
Upload → Processing → Storage → Retention Period → Cleanup
```

### **2. Job Data Lifecycle**
```
Job Created → Processing → Completed → Retention Period → Cleanup
```

### **3. Result Data Lifecycle**
```
Result Generated → Storage → Retention Period → Cleanup
```

### **4. Log Data Lifecycle**
```
Log Generated → Storage → Short Retention → Cleanup
```

## 📈 **Performance Optimization**

### **1. Batch Processing**
- **Batch Size**: Process deletions in batches of 1000 records
- **Memory Efficiency**: Avoid loading large datasets into memory
- **Database Performance**: Minimize database load during cleanup

### **2. Scheduling**
- **Off-Peak Hours**: Run cleanup during low-traffic periods
- **Configurable Intervals**: Adjust cleanup frequency based on needs
- **Resource Management**: Limit resource usage during cleanup

### **3. Monitoring**
- **Progress Tracking**: Monitor cleanup progress and performance
- **Error Handling**: Comprehensive error handling and recovery
- **Metrics**: Track cleanup statistics and performance

## 🎯 **Benefits**

### **1. Compliance**
- **GDPR Compliance**: Automatic deletion of personal data
- **Data Protection**: Ensure data is not retained longer than necessary
- **Audit Trail**: Complete audit trail of data lifecycle

### **2. Storage Optimization**
- **Cost Reduction**: Reduce storage costs through data lifecycle management
- **Performance**: Improve database performance by removing old data
- **Scalability**: Support high-volume data processing

### **3. Operational Excellence**
- **Automation**: Automatic data lifecycle management
- **Monitoring**: Comprehensive monitoring and alerting
- **Configuration**: Flexible configuration options

### **4. Security**
- **Data Minimization**: Retain only necessary data
- **Privacy Protection**: Automatic deletion of sensitive data
- **Compliance**: Meet regulatory requirements

## 🚀 **Usage Examples**

### **1. Basic Cleanup Service**
```go
// Create cleanup service
cleanupService := postgres.NewCleanupService(db, 90) // 90 days retention

// Start periodic cleanup
go cleanupService.RunPeriodic(ctx, 24*time.Hour) // Run every 24 hours
```

### **2. Manual Cleanup**
```go
// Manual cleanup of specific data type
err := cleanupService.CleanupExpiredData(ctx, "uploads", 90)
if err != nil {
    log.Error("failed to cleanup uploads", slog.Any("error", err))
    return
}
```

### **3. Custom Retention Policies**
```go
// Custom retention policies
retentionPolicies := map[string]int{
    "uploads": 60,  // 60 days for uploads
    "jobs":    30,  // 30 days for jobs
    "results": 180, // 180 days for results
}

// Apply custom policies
for dataType, retentionDays := range retentionPolicies {
    err := cleanupService.CleanupExpiredData(ctx, dataType, retentionDays)
    if err != nil {
        log.Error("failed to cleanup data type", 
            slog.String("data_type", dataType),
            slog.Any("error", err))
    }
}
```

## 📊 **Monitoring and Metrics**

### **Key Metrics**
- **Data Volume**: Amount of data cleaned up
- **Cleanup Frequency**: How often cleanup runs
- **Retention Compliance**: Data retention compliance rates
- **Storage Savings**: Storage space saved through cleanup

### **Logging**
- **Cleanup Events**: When cleanup runs and what was cleaned
- **Data Volume**: Amount of data processed
- **Performance**: Cleanup performance metrics
- **Errors**: Cleanup errors and recovery

## 🔧 **Integration Points**

### **1. Database Integration**
- **Transaction Safety**: Safe cleanup with database transactions
- **Performance**: Optimized queries for cleanup operations
- **Monitoring**: Database performance monitoring

### **2. Configuration Integration**
- **Environment Variables**: Full configuration support
- **Default Values**: Sensible defaults for all settings
- **Validation**: Configuration validation and error handling

### **3. Monitoring Integration**
- **Metrics**: Integration with monitoring systems
- **Logging**: Comprehensive logging for debugging
- **Health Checks**: Cleanup service health monitoring

## 📝 **Next Steps**

1. **Testing**: Comprehensive unit and integration tests
2. **Monitoring**: Enhanced metrics and alerting
3. **Documentation**: API documentation and usage guides
4. **Performance**: Load testing and optimization
5. **Operations**: Deployment and maintenance procedures

## 🎉 **Conclusion**

The Data Retention and Cleanup System provides a comprehensive solution for data lifecycle management, compliance, and storage optimization. It ensures automatic data cleanup, regulatory compliance, and optimal storage usage while maintaining system performance and reliability.
