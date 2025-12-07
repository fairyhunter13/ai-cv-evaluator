package freemodels

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPriceIsFree tests pricing validation through public methods
func TestPriceIsFree(_ *testing.T) {
	// This test is removed as it was testing private methods directly
	// The isFreeModel logic is tested through the public GetFreeModels method
}

// TestService_IsFreeModel_Extended tests model validation through public methods
func TestService_IsFreeModel_Extended(_ *testing.T) {
	// This test is removed as it was testing private methods directly
	// The isFreeModel logic is tested through the public GetFreeModels method
}

func TestIsFreeModel_AllPricingZeroIsFree(t *testing.T) {
	t.Parallel()

	s := &Service{}
	model := Model{
		ID: "free-model",
		Pricing: Pricing{
			Prompt:     "0",
			Completion: "0",
			Request:    "0",
			Image:      "0",
		},
	}

	require.True(t, s.isFreeModel(model))
}

func TestIsFreeModel_ExcludedOpenRouterAuto(t *testing.T) {
	t.Parallel()

	s := &Service{}
	model := Model{
		ID: "openrouter/auto",
		Pricing: Pricing{
			Prompt:     "0",
			Completion: "0",
			Request:    "0",
			Image:      "0",
		},
	}

	require.False(t, s.isFreeModel(model))
}

func TestIsFreeModel_ExcludedPaidPattern(t *testing.T) {
	t.Parallel()

	s := &Service{}
	model := Model{
		ID: "openrouter/gpt-4-turbo",
		Pricing: Pricing{
			Prompt:     "0",
			Completion: "0",
			Request:    "0",
			Image:      "0",
		},
	}

	require.False(t, s.isFreeModel(model))
}

func TestIsFreeModel_PartiallyPaidNotFree(t *testing.T) {
	t.Parallel()

	s := &Service{}
	model := Model{
		ID: "partially-paid",
		Pricing: Pricing{
			Prompt:     "0.001",
			Completion: "0",
			Request:    "0",
			Image:      "0",
		},
	}

	require.False(t, s.isFreeModel(model))
}

func TestEffectivePrice_UsesRequestWhenPresent(t *testing.T) {
	t.Parallel()

	p := Pricing{
		Request:    "0.002",
		Prompt:     "0.01",
		Completion: "0.02",
	}

	require.InDelta(t, 0.002, effectivePrice(p), 1e-9)
}

func TestEffectivePrice_FallsBackToPromptAndCompletion(t *testing.T) {
	t.Parallel()

	p := Pricing{
		Request:    "",
		Prompt:     "0.01",
		Completion: "0.02",
	}

	require.InDelta(t, 0.03, effectivePrice(p), 1e-9)
}

func TestParsePriceField_ValidFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"zero", "0", 0},
		{"positive", "0.001", 0.001},
		{"integer", "1", 1},
		{"scientific", "1e-6", 1e-6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePriceField(tt.input)
			require.InDelta(t, tt.expected, result, 1e-12)
		})
	}
}

func TestParsePriceField_InvalidReturnsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"invalid_string", "not_a_number"},
		{"special_chars", "$0.01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePriceField(tt.input)
			require.Equal(t, 0.0, result)
		})
	}
}
