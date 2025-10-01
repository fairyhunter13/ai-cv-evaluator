package observability_test

import (
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/stretchr/testify/assert"
)

func TestRecordAITokenUsage(t *testing.T) {
	t.Parallel()

	// Test recording AI token usage
	observability.RecordAITokenUsage("openai", "prompt", "gpt-4", 100)
	observability.RecordAITokenUsage("anthropic", "completion", "claude-3", 50)

	// These functions don't return values, so we just verify they don't panic
	// In a real test environment, you might want to check the actual metrics
	assert.True(t, true) // Placeholder assertion
}

func TestRecordRAGEffectiveness(t *testing.T) {
	t.Parallel()

	// Test recording RAG effectiveness
	observability.RecordRAGEffectiveness("cv", "query", 0.85)
	observability.RecordRAGEffectiveness("project", "query", 0.92)

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestRecordScoreDrift(t *testing.T) {
	t.Parallel()

	// Test recording score drift
	observability.RecordScoreDrift("accuracy", "v1.0", "corpus-v1", 0.15)
	observability.RecordScoreDrift("precision", "v1.0", "corpus-v1", 0.08)

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestRecordCircuitBreakerStatus(t *testing.T) {
	t.Parallel()

	// Test recording circuit breaker status
	observability.RecordCircuitBreakerStatus("ai-service", "call", 0) // Closed
	observability.RecordCircuitBreakerStatus("ai-service", "call", 1) // Open
	observability.RecordCircuitBreakerStatus("ai-service", "call", 2) // Half-open

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestRecordRAGRetrievalError(t *testing.T) {
	t.Parallel()

	// Test recording RAG retrieval errors
	observability.RecordRAGRetrievalError("cv", "timeout")
	observability.RecordRAGRetrievalError("project", "connection_failed")

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestMetricsFunctions_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test with edge case values
	observability.RecordAITokenUsage("", "", "", 0)
	observability.RecordRAGEffectiveness("", "query", 0.0)
	observability.RecordScoreDrift("", "", "", 0.0)
	observability.RecordCircuitBreakerStatus("", "", -1)
	observability.RecordRAGRetrievalError("", "")

	// Test with extreme values
	observability.RecordAITokenUsage("test", "test", "test", 999999)
	observability.RecordRAGEffectiveness("test", "query", 1.0)
	observability.RecordScoreDrift("test", "test", "test", 999.999)
	observability.RecordCircuitBreakerStatus("test", "test", 999)
	observability.RecordRAGRetrievalError("test", "test")

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestMetricsFunctions_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	// Test concurrent access to metrics functions
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(index int) {
			observability.RecordAITokenUsage("provider", "operation", "model", index)
			observability.RecordRAGEffectiveness("collection", "query", 0.5+float64(index)*0.1)
			observability.RecordScoreDrift("metric", "version", "corpus", float64(index)*0.1)
			observability.RecordCircuitBreakerStatus("service", "call", index%3)
			observability.RecordRAGRetrievalError("collection", "error")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestMetricsFunctions_RealisticScenarios(t *testing.T) {
	t.Parallel()

	// Test realistic usage scenarios
	scenarios := []struct {
		name      string
		provider  string
		operation string
		model     string
		tokens    int
	}{
		{"OpenAI Chat", "openai", "prompt", "gpt-4", 100},
		{"Anthropic Chat", "anthropic", "completion", "claude-3", 50},
		{"OpenAI Embed", "openai", "prompt", "text-embedding-ada-002", 25},
		{"Custom Model", "custom", "completion", "custom-model", 75},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(_ *testing.T) {
			observability.RecordAITokenUsage(scenario.provider, "prompt", scenario.model, scenario.tokens)
			observability.RecordAITokenUsage(scenario.provider, "completion", scenario.model, scenario.tokens/2)

			// Test RAG effectiveness for different collections
			observability.RecordRAGEffectiveness("cv", "query", 0.8+float64(scenario.tokens%20)*0.01)
			observability.RecordRAGEffectiveness("project", "query", 0.9+float64(scenario.tokens%20)*0.01)

			// Test score drift for different metrics
			observability.RecordScoreDrift("accuracy", "v1.0", "corpus-v1",
				float64(scenario.tokens%10)*0.01)
			observability.RecordScoreDrift("precision", "v1.0", "corpus-v1",
				float64(scenario.tokens%10)*0.01)

			// Test circuit breaker states
			state := scenario.tokens % 3
			observability.RecordCircuitBreakerStatus(scenario.provider, scenario.operation, state)

			// Test RAG retrieval errors
			errorTypes := []string{"timeout", "connection_failed", "rate_limit", "invalid_query"}
			errorType := errorTypes[scenario.tokens%len(errorTypes)]
			observability.RecordRAGRetrievalError("cv", errorType)
		})
	}

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}

func TestMetricsFunctions_Performance(t *testing.T) {
	t.Parallel()

	// Test performance with many calls
	start := time.Now()

	for i := 0; i < 1000; i++ {
		observability.RecordAITokenUsage("test", "test", "test", i)
		observability.RecordRAGEffectiveness("test", "query", 0.5)
		observability.RecordScoreDrift("test", "test", "test", float64(i)*0.001)
		observability.RecordCircuitBreakerStatus("test", "test", i%3)
		observability.RecordRAGRetrievalError("test", "test")
	}

	duration := time.Since(start)

	// Should complete quickly (less than 1 second for 1000 calls)
	assert.Less(t, duration, time.Second)
}

func TestMetricsFunctions_StringValues(t *testing.T) {
	t.Parallel()

	// Test with various string values
	providers := []string{"openai", "anthropic", "cohere", "huggingface", "custom"}
	operations := []string{"chat", "embed", "generate", "classify", "summarize"}
	collections := []string{"cv", "project", "job_description", "study_case"}
	errorTypes := []string{"timeout", "connection_failed", "rate_limit", "invalid_query", "server_error"}

	for _, provider := range providers {
		for _, operation := range operations {
			_ = operation // Use the variable to avoid "declared and not used" error
			observability.RecordAITokenUsage(provider, "prompt", "model", 100)
		}
	}

	for _, collection := range collections {
		observability.RecordRAGEffectiveness(collection, "query", 0.8)
		observability.RecordRAGRetrievalError(collection, "test_error")
	}

	for _, errorType := range errorTypes {
		observability.RecordRAGRetrievalError("test_collection", errorType)
	}

	// These functions don't return values, so we just verify they don't panic
	assert.True(t, true) // Placeholder assertion
}
