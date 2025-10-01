package shared_test

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	"github.com/stretchr/testify/assert"
)

func TestSearchResult(t *testing.T) {
	t.Parallel()

	// Test SearchResult struct
	result := shared.SearchResult{
		Text:  "test text",
		Score: 0.85,
	}

	assert.Equal(t, "test text", result.Text)
	assert.Equal(t, 0.85, result.Score)
}

func TestSearchResult_Empty(t *testing.T) {
	t.Parallel()

	// Test empty SearchResult
	result := shared.SearchResult{}

	assert.Empty(t, result.Text)
	assert.Equal(t, 0.0, result.Score)
}

func TestSearchResult_ZeroScore(t *testing.T) {
	t.Parallel()

	// Test SearchResult with zero score
	result := shared.SearchResult{
		Text:  "zero score text",
		Score: 0.0,
	}

	assert.Equal(t, "zero score text", result.Text)
	assert.Equal(t, 0.0, result.Score)
}

func TestSearchResult_HighScore(t *testing.T) {
	t.Parallel()

	// Test SearchResult with high score
	result := shared.SearchResult{
		Text:  "high score text",
		Score: 0.99,
	}

	assert.Equal(t, "high score text", result.Text)
	assert.Equal(t, 0.99, result.Score)
}

func TestSearchResult_NegativeScore(t *testing.T) {
	t.Parallel()

	// Test SearchResult with negative score
	result := shared.SearchResult{
		Text:  "negative score text",
		Score: -0.5,
	}

	assert.Equal(t, "negative score text", result.Text)
	assert.Equal(t, -0.5, result.Score)
}

func TestSearchResult_MultipleResults(t *testing.T) {
	t.Parallel()

	// Test multiple SearchResults
	results := []shared.SearchResult{
		{Text: "result 1", Score: 0.8},
		{Text: "result 2", Score: 0.9},
		{Text: "result 3", Score: 0.7},
	}

	assert.Len(t, results, 3)
	assert.Equal(t, "result 1", results[0].Text)
	assert.Equal(t, 0.8, results[0].Score)
	assert.Equal(t, "result 2", results[1].Text)
	assert.Equal(t, 0.9, results[1].Score)
	assert.Equal(t, "result 3", results[2].Text)
	assert.Equal(t, 0.7, results[2].Score)
}

func TestSearchResult_Context(t *testing.T) {
	t.Parallel()

	// Test SearchResult in context
	ctx := context.Background()

	result := shared.SearchResult{
		Text:  "contextual text",
		Score: 0.75,
	}

	// Verify the result can be used in context
	assert.NotNil(t, ctx)
	assert.Equal(t, "contextual text", result.Text)
	assert.Equal(t, 0.75, result.Score)
}

func TestSearchResult_Comparison(t *testing.T) {
	t.Parallel()

	// Test comparing SearchResults
	result1 := shared.SearchResult{Text: "text1", Score: 0.8}
	result2 := shared.SearchResult{Text: "text2", Score: 0.9}
	result3 := shared.SearchResult{Text: "text1", Score: 0.8}

	// Different text, different score
	assert.NotEqual(t, result1, result2)

	// Same text, different score
	assert.NotEqual(t, result1, result2)

	// Same text, same score
	assert.Equal(t, result1, result3)
}

func TestSearchResult_EdgeCases(t *testing.T) {
	t.Parallel()

	// Test edge cases
	tests := []struct {
		name  string
		text  string
		score float64
	}{
		{"empty text", "", 0.5},
		{"very long text", "a" + string(make([]byte, 1000)), 0.5},
		{"unicode text", "测试文本", 0.5},
		{"special characters", "!@#$%^&*()", 0.5},
		{"newlines", "line1\nline2\nline3", 0.5},
		{"tabs", "col1\tcol2\tcol3", 0.5},
		{"spaces", "   spaced   text   ", 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shared.SearchResult{
				Text:  tt.text,
				Score: tt.score,
			}
			assert.Equal(t, tt.text, result.Text)
			assert.Equal(t, tt.score, result.Score)
		})
	}
}

func TestSearchResult_ScoreRanges(t *testing.T) {
	t.Parallel()

	// Test various score ranges
	scoreTests := []float64{
		0.0, 0.1, 0.5, 0.9, 1.0,
		-1.0, -0.5, -0.1,
		1.5, 2.0, 10.0,
		-10.0, -2.0, -1.5,
	}

	for _, score := range scoreTests {
		t.Run("score_"+string(rune(score)), func(t *testing.T) {
			result := shared.SearchResult{
				Text:  "test",
				Score: score,
			}
			assert.Equal(t, score, result.Score)
		})
	}
}
