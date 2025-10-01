package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResponseCleaner(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()
	assert.NotNil(t, cleaner)
}

func TestResponseCleaner_CleanJSONResponse(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean_json",
			input:    `{"status": "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "markdown_wrapped_json",
			input:    "```json\n{\"status\": \"success\"}\n```",
			expected: `{"status": "success"}`,
		},
		{
			name:     "mixed_content_with_json",
			input:    "Here is the response: {\"status\": \"success\", \"data\": \"test\"}",
			expected: `{"status": "success", "data": "test"}`,
		},
		{
			name:     "json_with_single_quotes",
			input:    "{'status': 'success', 'data': 'test'}",
			expected: `{"status": "success", "data": "test"}`,
		},
		{
			name:     "json_with_backticks",
			input:    "{`status`: `success`, `data`: `test`}",
			expected: `{"status": "success", "data": "test"}`,
		},
		{
			name:     "json_with_markdown_formatting",
			input:    "{**status**: **success**, *data*: *test*}",
			expected: `{"status": "success", "data": "test"}`,
		},
		{
			name:     "json_with_trailing_comma",
			input:    `{"status": "success", "data": "test",}`,
			expected: `{"status": "success", "data": "test"}`,
		},
		{
			name:     "json_with_unquoted_keys",
			input:    `{status: "success", data: "test"}`,
			expected: `{"status": "success", "data": "test"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := cleaner.CleanJSONResponse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_removeMarkdownBlocks(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "json_markdown_block",
			input:    "```json\n{\"status\": \"success\"}\n```",
			expected: `{"status": "success"}`,
		},
		{
			name:     "generic_markdown_block",
			input:    "```\n{\"status\": \"success\"}\n```",
			expected: `{"status": "success"}`,
		},
		{
			name:     "no_markdown",
			input:    `{"status": "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "multiple_blocks",
			input:    "```json\n{\"status\": \"success\"}\n```\n```\n{\"data\": \"test\"}\n```",
			expected: "{\"status\": \"success\"}\n```\n```\n{\"data\": \"test\"}",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.removeMarkdownBlocks(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_fixJSONFormatting(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "backticks_to_quotes",
			input:    "{`status`: `success`}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "single_quotes_to_double",
			input:    "{'status': 'success'}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "bold_markdown",
			input:    "{**status**: **success**}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "italic_markdown",
			input:    "{*status*: *success*}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "mixed_formatting",
			input:    "{`status`: 'success', **data**: *test*}",
			expected: `{"status": "success", "data": "test"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.fixJSONFormatting(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_extractJSON(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure_json",
			input:    `{"status": "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "json_with_prefix",
			input:    "Here is the result: {\"status\": \"success\"}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "json_with_suffix",
			input:    `{"status": "success"} - end of response`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "nested_json",
			input:    "Result: {\"data\": {\"status\": \"success\"}}",
			expected: `{"data": {"status": "success"}}`,
		},
		{
			name:     "no_json",
			input:    "This is just text",
			expected: "This is just text",
		},
		{
			name:     "multiple_objects",
			input:    "First: {\"a\": 1} Second: {\"b\": 2}",
			expected: `{"a": 1}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.extractJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_validateAndFixJSON(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid_json",
			input:    `{"status": "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "trailing_comma",
			input:    `{"status": "success",}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "unquoted_keys",
			input:    `{status: "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "single_quotes",
			input:    `{'status': 'success'}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "prefix_text",
			input:    "Response: {\"status\": \"success\"}",
			expected: `{"status": "success"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.validateAndFixJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_fixCommonJSONIssues(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "trailing_comma",
			input:    `{"status": "success",}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "unquoted_keys",
			input:    `{status: "success"}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "single_quotes",
			input:    `{'status': 'success'}`,
			expected: `{"status": "success"}`,
		},
		{
			name:     "prefix_text",
			input:    "Result: {\"status\": \"success\"}",
			expected: `{"status": "success"}`,
		},
		{
			name:     "multiple_trailing_commas",
			input:    `{"a": 1, "b": 2,}`,
			expected: `{"a": 1, "b": 2}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.fixCommonJSONIssues(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_IsValidJSON(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid_json",
			input:    `{"status": "success"}`,
			expected: true,
		},
		{
			name:     "invalid_json",
			input:    `{status: success}`,
			expected: false,
		},
		{
			name:     "empty_string",
			input:    "",
			expected: false,
		},
		{
			name:     "valid_array",
			input:    `[{"id": 1}, {"id": 2}]`,
			expected: true,
		},
		{
			name:     "malformed_json",
			input:    `{"status": "success",}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cleaner.IsValidJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResponseCleaner_CleanAndValidateJSON(t *testing.T) {
	t.Parallel()

	cleaner := NewResponseCleaner()

	tests := []struct {
		name          string
		input         string
		expected      string
		expectedError bool
	}{
		{
			name:          "valid_cleanable_json",
			input:         "```json\n{\"status\": \"success\"}\n```",
			expected:      `{"status": "success"}`,
			expectedError: false,
		},
		{
			name:          "invalid_json_after_cleaning",
			input:         "This is not JSON at all",
			expected:      "",
			expectedError: true,
		},
		{
			name:          "valid_json_with_fixes",
			input:         `{'status': 'success'}`,
			expected:      `{"status": "success"}`,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := cleaner.CleanAndValidateJSON(tt.input)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestJSONValidationError_Error(t *testing.T) {
	t.Parallel()

	err := &JSONValidationError{
		Original: "original",
		Cleaned:  "cleaned",
		Message:  "test error",
	}

	assert.Equal(t, "test error", err.Error())
}
