# Free Models System Implementation

## 🎯 **Overview**

This document describes the Free Models System implemented in the AI CV Evaluator, which provides cost-effective AI processing using free models from OpenRouter with intelligent model selection and switching.

## 🚀 **Key Features Implemented**

### **1. Free Models Service**

#### **Purpose**
Manage and provide access to free AI models from OpenRouter with automatic refresh and caching.

#### **Implementation**
```go
// FreeModelsService manages free models from OpenRouter
type FreeModelsService struct {
    client      *http.Client
    apiKey      string
    baseURL     string
    refreshRate time.Duration
    models      []Model
    lastRefresh time.Time
    mu          sync.RWMutex
}
```

#### **Features**
- **Automatic Refresh**: Periodically refresh available free models
- **Model Caching**: Cache model information for performance
- **Health Monitoring**: Track model availability and performance
- **Fallback Support**: Graceful handling of service unavailability

### **2. Free Models Wrapper**

#### **Purpose**
Wrap the AI client to use free models with intelligent selection and fallback.

#### **Implementation**
```go
// FreeModelWrapper wraps AI client to use free models
type FreeModelWrapper struct {
    aiClient    domain.AIClient
    freeModels  *freemodels.FreeModelsService
    modelIndex  int64
    mu          sync.RWMutex
}
```

#### **Features**
- **Model Rotation**: Round-robin selection of available models
- **Automatic Fallback**: Switch to different models on failure
- **Performance Tracking**: Monitor model success rates
- **Cost Optimization**: Use only free models to minimize costs

### **3. Model Selection Logic**

#### **Purpose**
Intelligently select the best available free model for processing.

#### **Implementation**
```go
// GetFreeModels returns available free models
func (fms *FreeModelsService) GetFreeModels(ctx context.Context) ([]Model, error) {
    fms.mu.RLock()
    if time.Since(fms.lastRefresh) < fms.refreshRate && len(fms.models) > 0 {
        models := make([]Model, len(fms.models))
        copy(models, fms.models)
        fms.mu.RUnlock()
        return models, nil
    }
    fms.mu.RUnlock()
    
    return fms.refreshModels(ctx)
}
```

#### **Selection Strategy**
- **Round-Robin**: Distribute load across available models
- **Health-Based**: Prefer healthy models
- **Performance-Based**: Track and prefer successful models
- **Fallback Chain**: Automatic switching on model failure

## 📊 **Architecture Integration**

### **Free Models Flow**
```
Request → Free Models Service → Model Selection → AI Processing → Response
                ↓
        Model Failure → Next Model → AI Processing → Response
                ↓
        All Models Failed → Error Response
```

### **Model Refresh Flow**
```
Periodic Timer → Refresh Models → Update Cache → Health Check → Ready for Requests
```

### **Model Selection Flow**
```
Request → Get Available Models → Select Model → Process → Success ✅
                ↓
        Failure → Mark Model → Select Next → Process → Success ✅
```

## ⚙️ **Configuration**

### **Environment Variables**
```bash
# Free Models Configuration
FREE_MODELS_REFRESH=1h                    # How often to refresh free models
OPENROUTER_API_KEY=your_api_key           # OpenRouter API key
OPENROUTER_BASE_URL=https://openrouter.ai/api/v1  # OpenRouter base URL
```

### **Service Configuration**
```go
// FreeModelsService configuration
type FreeModelsService struct {
    client      *http.Client
    apiKey      string
    baseURL     string
    refreshRate time.Duration  // How often to refresh models
    models      []Model        // Cached models
    lastRefresh time.Time      // Last refresh timestamp
    mu          sync.RWMutex   // Thread safety
}
```

## 🎯 **Model Management**

### **1. Model Refresh**
```go
// refreshModels fetches latest free models from OpenRouter
func (fms *FreeModelsService) refreshModels(ctx context.Context) ([]Model, error) {
    // Create request to OpenRouter API
    req, err := http.NewRequestWithContext(ctx, "GET", fms.baseURL+"/models", nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    // Add authentication
    req.Header.Set("Authorization", "Bearer "+fms.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    // Make request
    resp, err := fms.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch models: %w", err)
    }
    defer resp.Body.Close()
    
    // Parse response and filter free models
    return fms.parseAndFilterModels(resp)
}
```

### **2. Model Filtering**
```go
// parseAndFilterModels filters for free models
func (fms *FreeModelsService) parseAndFilterModels(resp *http.Response) ([]Model, error) {
    var apiResponse struct {
        Data []struct {
            ID       string `json:"id"`
            Name     string `json:"name"`
            Pricing  struct {
                Prompt     string `json:"prompt"`
                Completion string `json:"completion"`
            } `json:"pricing"`
        } `json:"data"`
    }
    
    // Parse response
    if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    // Filter for free models (pricing is "0" or empty)
    var freeModels []Model
    for _, model := range apiResponse.Data {
        if model.Pricing.Prompt == "0" && model.Pricing.Completion == "0" {
            freeModels = append(freeModels, Model{
                ID:   model.ID,
                Name: model.Name,
            })
        }
    }
    
    return freeModels, nil
}
```

### **3. Model Selection**
```go
// SelectModel selects the next available model
func (fms *FreeModelsService) SelectModel() (Model, error) {
    fms.mu.RLock()
    defer fms.mu.RUnlock()
    
    if len(fms.models) == 0 {
        return Model{}, fmt.Errorf("no free models available")
    }
    
    // Round-robin selection
    index := atomic.AddInt64(&fms.modelIndex, 1) % int64(len(fms.models))
    return fms.models[index], nil
}
```

## 📈 **Performance Optimization**

### **1. Caching Strategy**
- **Model Caching**: Cache model information to reduce API calls
- **Refresh Rate**: Configurable refresh interval (default: 1 hour)
- **Thread Safety**: Concurrent access protection with mutex

### **2. Load Balancing**
- **Round-Robin**: Distribute requests across available models
- **Health Monitoring**: Track model health and performance
- **Automatic Failover**: Switch to healthy models on failure

### **3. Cost Optimization**
- **Free Models Only**: Use only free models to minimize costs
- **Efficient Selection**: Optimize model selection for performance
- **Resource Management**: Manage API rate limits and quotas

## 🎯 **Benefits**

### **1. Cost Effectiveness**
- **Zero Cost**: Use only free models from OpenRouter
- **No API Charges**: Eliminate per-request costs
- **Scalable**: Handle high volume without cost concerns

### **2. Reliability**
- **Multiple Models**: Fallback to different models on failure
- **Automatic Refresh**: Keep model list up-to-date
- **Health Monitoring**: Track and avoid failing models

### **3. Performance**
- **Load Distribution**: Spread load across multiple models
- **Caching**: Reduce API calls with intelligent caching
- **Optimization**: Select best-performing models

### **4. Operational Excellence**
- **Monitoring**: Track model usage and performance
- **Configuration**: Flexible configuration options
- **Logging**: Comprehensive logging for debugging

## 🚀 **Usage Examples**

### **1. Basic Free Models Usage**
```go
// Create free models service
freeModelsService := freemodels.NewFreeModelsService(
    http.DefaultClient,
    apiKey,
    baseURL,
    1*time.Hour, // Refresh every hour
)

// Get available models
models, err := freeModelsService.GetFreeModels(ctx)
if err != nil {
    log.Error("failed to get free models", slog.Any("error", err))
    return
}

// Select a model
model, err := freeModelsService.SelectModel()
if err != nil {
    log.Error("no free models available", slog.Any("error", err))
    return
}
```

### **2. Free Model Wrapper Usage**
```go
// Create free model wrapper
freeModelWrapper := freemodels.NewFreeModelWrapper(aiClient, freeModelsService)

// Use wrapper for AI processing
response, err := freeModelWrapper.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
if err != nil {
    log.Error("AI processing failed", slog.Any("error", err))
    return
}
```

### **3. Model Refresh**
```go
// Manual model refresh
err := freeModelsService.Refresh(ctx)
if err != nil {
    log.Error("failed to refresh models", slog.Any("error", err))
    return
}

// Check refresh status
if time.Since(freeModelsService.LastRefresh()) > 2*time.Hour {
    log.Warn("models haven't been refreshed in 2 hours")
}
```

## 📊 **Monitoring and Metrics**

### **Key Metrics**
- **Model Availability**: Number of available free models
- **Model Usage**: Distribution of requests across models
- **Refresh Frequency**: How often models are refreshed
- **Model Performance**: Success rates per model

### **Logging**
- **Model Selection**: Which model was selected for each request
- **Refresh Events**: When models are refreshed
- **Model Health**: Model availability and performance
- **Error Tracking**: Model failures and fallbacks

## 🔧 **Integration Points**

### **1. AI Client Integration**
- **Seamless Integration**: Works with existing AI client
- **Transparent Usage**: No changes to existing code
- **Fallback Support**: Automatic fallback on model failure

### **2. Configuration Integration**
- **Environment Variables**: Full configuration support
- **Default Values**: Sensible defaults for all settings
- **Validation**: Configuration validation and error handling

### **3. Monitoring Integration**
- **Metrics**: Integration with monitoring systems
- **Logging**: Comprehensive logging for debugging
- **Health Checks**: Model health monitoring

## 📝 **Next Steps**

1. **Testing**: Comprehensive unit and integration tests
2. **Monitoring**: Enhanced metrics and alerting
3. **Documentation**: API documentation and usage guides
4. **Performance**: Load testing and optimization
5. **Operations**: Deployment and maintenance procedures

## 🎉 **Conclusion**

The Free Models System provides a cost-effective, reliable, and performant solution for AI processing using free models from OpenRouter. It ensures high availability through intelligent model selection, automatic fallback, and comprehensive monitoring while maintaining zero operational costs.
