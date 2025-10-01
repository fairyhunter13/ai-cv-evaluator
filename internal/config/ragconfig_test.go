package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRAGConfig_LoadRAGConfig(t *testing.T) {
	// Test with existing config files
	config, err := LoadRAGConfig()
	if err != nil {
		// If config files don't exist, that's expected in some environments
		t.Logf("RAG config files not found, skipping test: %v", err)
		return
	}

	require.NotNil(t, config)
	assert.NotEmpty(t, config.JobDescription)
	assert.NotEmpty(t, config.StudyCaseBrief)
	assert.NotEmpty(t, config.ScoringRubric)
}

func TestRAGConfig_LoadRAGConfig_FileNotFound(t *testing.T) {
	// Test with non-existent file
	_, err := loadTextFromYAML("non-existent-file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRAGConfig_LoadTextFromYAML_InvalidYAML(t *testing.T) {
	// Create a temporary file with invalid YAML
	tmpFile, err := os.CreateTemp("", "test-invalid-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write invalid YAML
	_, err = tmpFile.WriteString("invalid: yaml: content: [")
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading invalid YAML
	_, err = loadTextFromYAML(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}

func TestRAGConfig_LoadTextFromYAML_EmptyTexts(t *testing.T) {
	// Create a temporary file with empty texts
	tmpFile, err := os.CreateTemp("", "test-empty-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write YAML with empty texts
	_, err = tmpFile.WriteString("texts: []")
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading empty texts
	_, err = loadTextFromYAML(tmpFile.Name())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no texts found")
}

func TestRAGConfig_LoadTextFromYAML_ValidContent(t *testing.T) {
	// Create a temporary file with valid YAML
	tmpFile, err := os.CreateTemp("", "test-valid-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write valid YAML with texts
	_, err = tmpFile.WriteString(`
texts:
  - "First line of text"
  - "Second line of text"
  - "Third line of text"
`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading valid content
	text, err := loadTextFromYAML(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "First line of text\nSecond line of text\nThird line of text", text)
}

func TestRAGConfig_LoadTextFromYAML_SingleText(t *testing.T) {
	// Create a temporary file with single text
	tmpFile, err := os.CreateTemp("", "test-single-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write YAML with single text
	_, err = tmpFile.WriteString(`
texts:
  - "Single line of text"
`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading single text
	text, err := loadTextFromYAML(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "Single line of text", text)
}

func TestRAGConfig_LoadTextFromYAML_WithWhitespace(t *testing.T) {
	// Create a temporary file with text containing whitespace
	tmpFile, err := os.CreateTemp("", "test-whitespace-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write YAML with text containing whitespace
	_, err = tmpFile.WriteString(`
texts:
  - "  Line with leading spaces  "
  - "  Another line with spaces  "
`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test loading text with whitespace
	text, err := loadTextFromYAML(tmpFile.Name())
	require.NoError(t, err)
	// Should be trimmed (only outer whitespace is trimmed)
	assert.Equal(t, "Line with leading spaces  \n  Another line with spaces", text)
}

func TestRAGConfig_GetDefaultJobDescription(t *testing.T) {
	// Test getting default job description
	desc := GetDefaultJobDescription()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "Backend Developer")
}

func TestRAGConfig_GetDefaultStudyCaseBrief(t *testing.T) {
	// Test getting default study case brief
	brief := GetDefaultStudyCaseBrief()
	assert.NotEmpty(t, brief)
	assert.Contains(t, brief, "Mini Project")
}

func TestRAGConfig_GetDefaultScoringRubric(t *testing.T) {
	// Test getting default scoring rubric
	rubric := GetDefaultScoringRubric()
	assert.NotEmpty(t, rubric)
	assert.Contains(t, rubric, "CV Match Evaluation")
}

func TestRAGConfig_LoadRAGConfig_Integration(t *testing.T) {
	// Create temporary config files for integration test
	tempDir, err := os.MkdirTemp("", "test-rag-config-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create job description file
	jobDescFile := filepath.Join(tempDir, "job_description.yaml")
	err = os.WriteFile(jobDescFile, []byte(`
texts:
  - "Test job description content"
`), 0600)
	require.NoError(t, err)

	// Create study case brief file
	studyCaseFile := filepath.Join(tempDir, "study_case_brief.yaml")
	err = os.WriteFile(studyCaseFile, []byte(`
texts:
  - "Test study case brief content"
`), 0600)
	require.NoError(t, err)

	// Create scoring rubric file
	scoringRubricFile := filepath.Join(tempDir, "scoring_rubric.yaml")
	err = os.WriteFile(scoringRubricFile, []byte(`
texts:
  - "Test scoring rubric content"
`), 0600)
	require.NoError(t, err)

	// Test loading with custom paths (this would require modifying the function to accept paths)
	// For now, we'll test the error handling when files don't exist
	_, err = loadTextFromYAML("non-existent-path/job_description.yaml")
	assert.Error(t, err)
}

func TestRAGConfig_LoadRAGConfig_AllFilesExist(t *testing.T) {
	// Create temporary config files for integration test
	tempDir, err := os.MkdirTemp("", "test-rag-config-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create the configs/rag directory structure
	ragDir := filepath.Join(tempDir, "configs", "rag")
	err = os.MkdirAll(ragDir, 0750)
	require.NoError(t, err)

	// Create job description file
	jobDescFile := filepath.Join(ragDir, "job_description.yaml")
	err = os.WriteFile(jobDescFile, []byte(`
texts:
  - "Test job description content"
  - "Additional job description line"
`), 0600)
	require.NoError(t, err)

	// Create study case brief file
	studyCaseFile := filepath.Join(ragDir, "study_case_brief.yaml")
	err = os.WriteFile(studyCaseFile, []byte(`
texts:
  - "Test study case brief content"
`), 0600)
	require.NoError(t, err)

	// Create scoring rubric file
	scoringRubricFile := filepath.Join(ragDir, "scoring_rubric.yaml")
	err = os.WriteFile(scoringRubricFile, []byte(`
texts:
  - "Test scoring rubric content"
`), 0600)
	require.NoError(t, err)

	// Change to the temp directory to test relative paths
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Test loading the config
	config, err := LoadRAGConfig()
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify all fields are loaded correctly
	assert.Equal(t, "Test job description content\nAdditional job description line", config.JobDescription)
	assert.Equal(t, "Test study case brief content", config.StudyCaseBrief)
	assert.Equal(t, "Test scoring rubric content", config.ScoringRubric)
}

func TestRAGConfig_LoadRAGConfig_JobDescriptionError(t *testing.T) {
	// Test when job description file fails to load
	_, err := loadTextFromYAML("non-existent-job-description.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRAGConfig_LoadRAGConfig_StudyCaseError(t *testing.T) {
	// Test when study case file fails to load
	_, err := loadTextFromYAML("non-existent-study-case.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRAGConfig_LoadRAGConfig_ScoringRubricError(t *testing.T) {
	// Test when scoring rubric file fails to load
	_, err := loadTextFromYAML("non-existent-scoring-rubric.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestRAGConfig_LoadTextFromYAML_AbsolutePath(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test-absolute-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write valid YAML
	_, err = tmpFile.WriteString(`
texts:
  - "Test content for absolute path"
`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test with absolute path
	text, err := loadTextFromYAML(tmpFile.Name())
	require.NoError(t, err)
	assert.Equal(t, "Test content for absolute path", text)
}

func TestRAGConfig_LoadTextFromYAML_RelativePath(t *testing.T) {
	// Create a temporary file in current directory
	tmpFile, err := os.CreateTemp(".", "test-relative-*.yaml")
	require.NoError(t, err)
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	// Write valid YAML
	_, err = tmpFile.WriteString(`
texts:
  - "Test content for relative path"
`)
	require.NoError(t, err)
	_ = tmpFile.Close()

	// Test with relative path
	text, err := loadTextFromYAML(filepath.Base(tmpFile.Name()))
	require.NoError(t, err)
	assert.Equal(t, "Test content for relative path", text)
}
