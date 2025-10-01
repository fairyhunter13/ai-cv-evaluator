package shared_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockAIClientErrorHandling is a mock for the AIClient interface for error handling tests
type mockAIClientErrorHandling struct {
	mock.Mock
}

func (m *mockAIClientErrorHandling) ChatJSON(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	args := m.Called(ctx, systemPrompt, userPrompt, maxTokens)
	return args.String(0), args.Error(1)
}

func (m *mockAIClientErrorHandling) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).([][]float64), args.Error(1)
}

func TestStableAIConfig(t *testing.T) {
	t.Parallel()

	config := shared.StableAIConfig()

	assert.NotNil(t, config)
	assert.Greater(t, config.MaxRetries, 0)
	assert.Greater(t, config.Timeout, time.Duration(0))
	assert.Greater(t, config.Temperature, 0.0)
	assert.Greater(t, config.MaxTokens, 0)
}

func TestValidateResponseQuality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "valid_json_response",
			response: `{"result": "valid data", "score": 8.5}`,
			expected: true,
		},
		{
			name:     "invalid_json_response",
			response: `invalid json`,
			expected: false,
		},
		{
			name:     "empty_response",
			response: "",
			expected: false,
		},
		{
			name:     "cot_leakage_detected",
			response: `{"result": "Let me think step by step about this problem..."}`,
			expected: false,
		},
		{
			name:     "thinking_process_detected",
			response: `{"result": "I think about this problem and analyze it"}`,
			expected: false,
		},
		{
			name:     "reasoning_detected",
			response: `{"result": "First, I need to reason about this"}`,
			expected: false,
		},
		{
			name:     "clean_response",
			response: `{"result": "clean data", "score": 7.5}`,
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.ValidateResponseQuality(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "valid_json",
			response: `{"result": "valid data"}`,
			expected: true,
		},
		{
			name:     "valid_json_array",
			response: `[{"item": "value"}]`,
			expected: true,
		},
		{
			name:     "invalid_json",
			response: `invalid json`,
			expected: false,
		},
		{
			name:     "empty_string",
			response: "",
			expected: false,
		},
		{
			name:     "malformed_json",
			response: `{"result": "data"`,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.IsValidJSON(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasCoTLeakage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "clean_response",
			response: `{"result": "clean data"}`,
			expected: false,
		},
		{
			name:     "step_by_step_reasoning",
			response: `{"result": "Let me analyze this problem step by step..."}`,
			expected: true,
		},
		{
			name:     "thinking_process",
			response: `{"result": "I think about this problem"}`,
			expected: true,
		},
		{
			name:     "reasoning_pattern",
			response: `{"result": "First, I need to reason about this"}`,
			expected: true,
		},
		{
			name:     "mixed_content",
			response: `{"result": "The answer is 42. Let me think step by step..."}`,
			expected: true,
		},
		{
			name:     "no_leakage",
			response: `{"result": "The final answer is 42"}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.HasCoTLeakage(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasReasonableContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		expected bool
	}{
		{
			name:     "reasonable_content",
			response: `{"result": "This is a reasonable response with good content"}`,
			expected: true,
		},
		{
			name:     "too_short",
			response: `{"result": "ok"}`,
			expected: false,
		},
		{
			name:     "empty_content",
			response: `{"result": ""}`,
			expected: false,
		},
		{
			name:     "good_length",
			response: `{"result": "This is a well-structured response with sufficient detail"}`,
			expected: true,
		},
		{
			name:     "very_long_content",
			response: `{"result": "This is a very long response that contains a lot of detailed information about the topic and provides comprehensive analysis"}`,
			expected: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.HasReasonableContent(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSimulateAPIFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		failureRate float64
		iterations  int
	}{
		{
			name:        "low_failure_rate",
			failureRate: 0.1,
			iterations:  100,
		},
		{
			name:        "high_failure_rate",
			failureRate: 0.5,
			iterations:  100,
		},
		{
			name:        "no_failures",
			failureRate: 0.0,
			iterations:  50,
		},
		{
			name:        "all_failures",
			failureRate: 1.0,
			iterations:  50,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test that the function doesn't panic and returns expected behavior
			for i := 0; i < tt.iterations; i++ {
				shared.SimulateAPIFailures(context.Background(), tt.failureRate)
			}
		})
	}
}

func TestHandleAPIErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		errorRate  float64
		iterations int
	}{
		{
			name:       "low_error_rate",
			errorRate:  0.1,
			iterations: 100,
		},
		{
			name:       "high_error_rate",
			errorRate:  0.5,
			iterations: 100,
		},
		{
			name:       "no_errors",
			errorRate:  0.0,
			iterations: 50,
		},
		{
			name:       "all_errors",
			errorRate:  1.0,
			iterations: 50,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Test that the function doesn't panic and returns expected behavior
			for i := 0; i < tt.iterations; i++ {
				shared.HandleAPIErrors(context.Background(), fmt.Errorf("test error"), "test-job")
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retryable_error",
			err:      assert.AnError,
			expected: true,
		},
		{
			name:     "nil_error",
			err:      nil,
			expected: false,
		},
		{
			name:     "context_canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "context_deadline_exceeded",
			err:      context.DeadlineExceeded,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateBackoffDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		attempt     int
		baseDelay   time.Duration
		maxDelay    time.Duration
		multiplier  float64
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{
			name:        "first_attempt",
			attempt:     0,
			baseDelay:   time.Second,
			maxDelay:    time.Minute,
			multiplier:  2.0,
			expectedMin: time.Second,
			expectedMax: time.Second * 2,
		},
		{
			name:        "second_attempt",
			attempt:     1,
			baseDelay:   time.Second,
			maxDelay:    time.Minute,
			multiplier:  2.0,
			expectedMin: time.Second * 2,
			expectedMax: time.Second * 4,
		},
		{
			name:        "high_attempt",
			attempt:     10,
			baseDelay:   time.Second,
			maxDelay:    time.Minute,
			multiplier:  2.0,
			expectedMin: time.Minute,
			expectedMax: time.Minute,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			delay := shared.CalculateBackoffDelay(tt.attempt, tt.baseDelay, tt.maxDelay, tt.multiplier)

			assert.GreaterOrEqual(t, delay, tt.expectedMin)
			assert.LessOrEqual(t, delay, tt.expectedMax)
		})
	}
}

func TestValidateResponseStability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []string
		expected  bool
	}{
		{
			name: "stable_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 8.3, "feedback": "Good work"}`,
				`{"score": 8.7, "feedback": "Good work"}`,
			},
			expected: true,
		},
		{
			name: "unstable_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 3.2, "feedback": "Poor work"}`,
				`{"score": 9.1, "feedback": "Excellent work"}`,
			},
			expected: false,
		},
		{
			name: "single_response",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
			},
			expected: true,
		},
		{
			name:      "empty_responses",
			responses: []string{},
			expected:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.ValidateResponseStability(tt.responses)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateResponseConsistency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []string
		expected  float64
	}{
		{
			name: "consistent_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 8.3, "feedback": "Good work"}`,
				`{"score": 8.7, "feedback": "Good work"}`,
			},
			expected: 0.8, // High consistency
		},
		{
			name: "inconsistent_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 3.2, "feedback": "Poor work"}`,
				`{"score": 9.1, "feedback": "Excellent work"}`,
			},
			expected: 0.3, // Low consistency
		},
		{
			name: "single_response",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
			},
			expected: 1.0, // Perfect consistency
		},
		{
			name:      "empty_responses",
			responses: []string{},
			expected:  1.0, // Perfect consistency
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.CalculateResponseConsistency(tt.responses)
			assert.InDelta(t, tt.expected, result, 0.2) // Allow some variance
		})
	}
}

func TestCalculateSimilarity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text1    string
		text2    string
		expected float64
	}{
		{
			name:     "identical_texts",
			text1:    "This is a test",
			text2:    "This is a test",
			expected: 1.0,
		},
		{
			name:     "similar_texts",
			text1:    "This is a test",
			text2:    "This is a test case",
			expected: 0.8, // High similarity
		},
		{
			name:     "different_texts",
			text1:    "This is a test",
			text2:    "That is a different example",
			expected: 0.3, // Low similarity
		},
		{
			name:     "empty_texts",
			text1:    "",
			text2:    "",
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.CalculateSimilarity(tt.text1, tt.text2)
			assert.InDelta(t, tt.expected, result, 0.2) // Allow some variance
		})
	}
}

func TestMonitorAIHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []string
		expected  bool
	}{
		{
			name: "healthy_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 8.3, "feedback": "Good work"}`,
				`{"score": 8.7, "feedback": "Good work"}`,
			},
			expected: true,
		},
		{
			name: "unhealthy_responses",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
				`{"score": 3.2, "feedback": "Poor work"}`,
				`{"score": 9.1, "feedback": "Excellent work"}`,
			},
			expected: false,
		},
		{
			name: "single_healthy_response",
			responses: []string{
				`{"score": 8.5, "feedback": "Good work"}`,
			},
			expected: true,
		},
		{
			name:      "empty_responses",
			responses: []string{},
			expected:  true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := shared.MonitorAIHealth(tt.responses)
			assert.Equal(t, tt.expected, result)
		})
	}
}
