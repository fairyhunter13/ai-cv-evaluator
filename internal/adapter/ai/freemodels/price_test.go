package freemodels

import (
	"testing"
)

func TestPriceIsFree(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: true,
		},
		{
			name:     "empty string",
			input:    "",
			expected: true,
		},
		{
			name:     "zero string",
			input:    "0",
			expected: true,
		},
		{
			name:     "zero decimal string",
			input:    "0.0",
			expected: true,
		},
		{
			name:     "whitespace string",
			input:    "  ",
			expected: true,
		},
		{
			name:     "zero float64",
			input:    float64(0),
			expected: true,
		},
		{
			name:     "zero int",
			input:    0,
			expected: false, // int 0 is not handled by priceIsFree function
		},
		{
			name:     "paid string",
			input:    "0.001",
			expected: false,
		},
		{
			name:     "paid float64",
			input:    float64(0.001),
			expected: false,
		},
		{
			name:     "paid int",
			input:    1,
			expected: false,
		},
		{
			name:     "non-zero string",
			input:    "1.5",
			expected: false,
		},
		{
			name:     "map with free nested value",
			input:    map[string]interface{}{"nested": "0"},
			expected: true,
		},
		{
			name:     "map with paid nested value",
			input:    map[string]interface{}{"nested": "0.001"},
			expected: false,
		},
		{
			name:     "map with mixed values - one free",
			input:    map[string]interface{}{"paid": "0.001", "free": "0"},
			expected: true,
		},
		{
			name:     "map with all paid values",
			input:    map[string]interface{}{"value1": "0.001", "value2": "0.002"},
			expected: false,
		},
		{
			name: "nested map with free value",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "0",
				},
			},
			expected: true,
		},
		{
			name: "nested map with all paid values",
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "0.001",
				},
			},
			expected: false,
		},
		{
			name:     "boolean false",
			input:    false,
			expected: false,
		},
		{
			name:     "boolean true",
			input:    true,
			expected: false,
		},
		{
			name:     "slice",
			input:    []string{"0"},
			expected: false,
		},
		{
			name:     "struct",
			input:    struct{}{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := priceIsFree(tt.input)
			if result != tt.expected {
				t.Errorf("priceIsFree(%#v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestService_IsFreeModel_Extended(t *testing.T) {
	service := New("test-key", "http://unused")

	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "free model with zero prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "0",
				},
			},
			expected: true,
		},
		{
			name: "free model with empty prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "",
				},
			},
			expected: true,
		},
		{
			name: "free model with nil prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: nil,
				},
			},
			expected: true,
		},
		{
			name: "free model with zero float prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: float64(0),
				},
			},
			expected: true,
		},
		{
			name: "paid model with non-zero prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: "0.001",
				},
			},
			expected: false,
		},
		{
			name: "paid model with non-zero float prompt price",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: float64(0.001),
				},
			},
			expected: false,
		},
		{
			name: "free model with nested free pricing",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: map[string]interface{}{"nested": "0"},
				},
			},
			expected: true,
		},
		{
			name: "paid model with nested paid pricing",
			model: Model{
				ID: "test-model",
				Pricing: Pricing{
					Prompt: map[string]interface{}{"nested": "0.001"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isFreeModel(tt.model)
			if result != tt.expected {
				t.Errorf("isFreeModel() = %v, want %v", result, tt.expected)
			}
		})
	}
}
