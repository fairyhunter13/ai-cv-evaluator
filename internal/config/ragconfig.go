// Package config provides configuration loading utilities for RAG configs.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// RAGConfig holds the configuration for RAG-related settings.
type RAGConfig struct {
	JobDescription string `yaml:"job_description"`
	StudyCaseBrief string `yaml:"study_case_brief"`
	ScoringRubric  string `yaml:"scoring_rubric"`
}

// RAGYAML represents the structure of RAG config YAML files.
type RAGYAML struct {
	Texts []string `yaml:"texts"`
}

// LoadRAGConfig loads RAG configuration from YAML files.
func LoadRAGConfig() (*RAGConfig, error) {
	config := &RAGConfig{}

	// Load job description
	jobDesc, err := loadTextFromYAML("configs/rag/job_description.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load job description: %w", err)
	}
	config.JobDescription = jobDesc

	// Load study case brief
	studyCase, err := loadTextFromYAML("configs/rag/study_case_brief.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load study case brief: %w", err)
	}
	config.StudyCaseBrief = studyCase

	// Load scoring rubric
	scoringRubric, err := loadTextFromYAML("configs/rag/scoring_rubric.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load scoring rubric: %w", err)
	}
	config.ScoringRubric = scoringRubric

	return config, nil
}

// loadTextFromYAML loads the first text entry from a RAG YAML file.
func loadTextFromYAML(filePath string) (string, error) {
	// Get absolute path to ensure we're reading from the correct location
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("config file not found: %s", absPath)
	}

	// Read file content
	// #nosec G304 -- Configuration files are expected to be safe
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var ragYAML RAGYAML
	if err := yaml.Unmarshal(content, &ragYAML); err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Extract the first text entry and clean it up
	if len(ragYAML.Texts) == 0 {
		return "", fmt.Errorf("no texts found in config file: %s", filePath)
	}

	// Join all texts with newlines and clean up
	text := strings.Join(ragYAML.Texts, "\n")
	text = strings.TrimSpace(text)

	// If the text is too long, we might want to truncate it for the default
	// For now, we'll use the full text
	return text, nil
}

// GetDefaultJobDescription returns the default job description from config.
func GetDefaultJobDescription() string {
	config, err := LoadRAGConfig()
	if err != nil {
		// Fallback to hardcoded value if config loading fails
		return "Backend Developer - APIs, DBs, cloud, prompt design, chaining and RAG."
	}
	return config.JobDescription
}

// GetDefaultStudyCaseBrief returns the default study case brief from config.
func GetDefaultStudyCaseBrief() string {
	config, err := LoadRAGConfig()
	if err != nil {
		// Fallback to hardcoded value if config loading fails
		return "Mini Project: Evaluate CV and project via AI workflow with retries and observability."
	}
	return config.StudyCaseBrief
}

// GetDefaultScoringRubric returns the default scoring rubric from config.
func GetDefaultScoringRubric() string {
	config, err := LoadRAGConfig()
	if err != nil {
		// Fallback to hardcoded value if config loading fails
		return "CV Match Evaluation (1–5 scale per parameter) and Project Deliverable Evaluation (1–5 scale per parameter)"
	}
	return config.ScoringRubric
}
