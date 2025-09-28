// Package asynqadp contains queue worker helpers for evaluation job processing.
package asynqadp

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type llmEvalOut struct {
	CVMatchRate     float64 `json:"cv_match_rate"`
	CVFeedback      string  `json:"cv_feedback"`
	ProjectScore    float64 `json:"project_score"`
	ProjectFeedback string  `json:"project_feedback"`
	OverallSummary  string  `json:"overall_summary"`
}

// Normalization pass prompts: provide previously parsed JSON and ask the model to re-emit
// strictly valid JSON clamped to the ranges and sentence limits.
func buildNormalizationSystemPrompt() string {
	return strings.TrimSpace(`You are a strict JSON normalizer. Input will be a JSON object that may contain minor schema issues.
Return ONLY valid JSON adhering to this schema and constraints (no markdown, no prose):
{
  "cv_match_rate": number between 0.0 and 1.0,
  "cv_feedback": string with 1-3 concise sentences,
  "project_score": number between 1.0 and 10.0,
  "project_feedback": string with 1-3 concise sentences,
  "overall_summary": string with 3-5 concise sentences
}`)
}

func buildNormalizationUserPrompt(o llmEvalOut) string {
	// Provide the first pass JSON as input, instructing to correct/clamp if needed.
	b, _ := json.Marshal(o)
	return "Please normalize this JSON, clamping numeric fields to ranges and enforcing sentence counts where possible. Return JSON only.\n" + string(b)
}

func validateEvalOut(o llmEvalOut) error {
	if o.CVMatchRate < 0 || o.CVMatchRate > 1 { return fmt.Errorf("cv_match_rate out of range") }
	if o.ProjectScore < 1 || o.ProjectScore > 10 { return fmt.Errorf("project_score out of range") }
	if strings.TrimSpace(o.CVFeedback) == "" { return fmt.Errorf("cv_feedback empty") }
	if strings.TrimSpace(o.ProjectFeedback) == "" { return fmt.Errorf("project_feedback empty") }
	if strings.TrimSpace(o.OverallSummary) == "" { return fmt.Errorf("overall_summary empty") }
	return nil
}

func isSchemaOrJSONErr(err error) bool {
	if err == nil { return false }
	s := err.Error()
	if strings.Contains(s, "invalid json") || strings.Contains(s, "schema invalid") || strings.Contains(s, "out of range") || strings.Contains(s, "empty") {
		return true
	}
	return false
}

func buildUserPromptRAG(cvText, projText, jobDesc, brief string, jobCtx, rubricCtx []string) string {
	b := &strings.Builder{}
	b.WriteString("Job Description (User input):\n")
	b.WriteString(truncate(jobDesc, 2000))
	if len(jobCtx) > 0 {
		b.WriteString("\nJob Description (Retrieved context):\n")
		for i, c := range jobCtx {
			if i >= 6 { break }
			b.WriteString("- ")
			b.WriteString(truncate(c, 800))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nStudy Case Brief (User input):\n")
	b.WriteString(truncate(brief, 2000))
	if len(rubricCtx) > 0 {
		b.WriteString("\nScoring Rubric (Retrieved context):\n")
		for i, c := range rubricCtx {
			if i >= 6 { break }
			b.WriteString("- ")
			b.WriteString(truncate(c, 800))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nCandidate CV Text:\n")
	b.WriteString(truncate(cvText, 3000))
	b.WriteString("\nProject Report Text:\n")
	b.WriteString(truncate(projText, 3000))
	b.WriteString("\nReturn JSON only.")
	return b.String()
}

func buildSystemPrompt() string {
	return strings.TrimSpace(`You are an expert technical evaluator. Return ONLY valid JSON that matches this schema and nothing else. No prose, no explanations.
JSON schema fields (all required):
{
  "cv_match_rate": number,   // between 0.0 and 1.0
  "cv_feedback": string,     // 1-3 concise sentences
  "project_score": number,   // between 1.0 and 10.0
  "project_feedback": string,// 1-3 concise sentences
  "overall_summary": string  // 3-5 concise sentences
}
You may reason privately but must return only JSON conforming to the schema. Do not include chain-of-thought.`)
}

func buildUserPrompt(cvText, projText, jobDesc, brief string) string {
	// Keep prompts compact; include key inputs.
	b := &strings.Builder{}
	b.WriteString("Job Description:\n")
	b.WriteString(truncate(jobDesc, 4000))
	b.WriteString("\nStudy Case Brief:\n")
	b.WriteString(truncate(brief, 4000))
	b.WriteString("\nCandidate CV Text:\n")
	b.WriteString(truncate(cvText, 4000))
	b.WriteString("\nProject Report Text:\n")
	b.WriteString(truncate(projText, 4000))
	b.WriteString("\nReturn JSON only.")
	return b.String()
}

func parseAndNormalize(s string) (llmEvalOut, error) {
	js, ok := extractFirstJSONObject(s)
	if !ok { return llmEvalOut{}, fmt.Errorf("invalid json: not found") }
	// Basic CoT leakage guard: reject obvious step-by-step patterns in fields
	if strings.Contains(js, "Step 1") || strings.Contains(js, "First,") || strings.Contains(js, "I think") {
		return llmEvalOut{}, fmt.Errorf("schema invalid: chain-of-thought detected")
	}
	var out llmEvalOut
	if err := json.Unmarshal([]byte(js), &out); err != nil {
		return llmEvalOut{}, fmt.Errorf("invalid json: %w", err)
	}
	// Clamp values
	if out.CVMatchRate < 0 { out.CVMatchRate = 0 }
	if out.CVMatchRate > 1 { out.CVMatchRate = 1 }
	if out.ProjectScore < 1 { out.ProjectScore = 1 }
	if out.ProjectScore > 10 { out.ProjectScore = 10 }
	// Trim texts
	out.CVFeedback = trimSentence(out.CVFeedback, 450)
	out.ProjectFeedback = trimSentence(out.ProjectFeedback, 450)
	out.OverallSummary = trimSentence(out.OverallSummary, 1200)
	// Enforce sentence count constraints: feedback 1-3 sentences; summary 3-5 sentences
	out.CVFeedback = limitSentences(out.CVFeedback, 1, 3)
	out.ProjectFeedback = limitSentences(out.ProjectFeedback, 1, 3)
	out.OverallSummary = limitSentences(out.OverallSummary, 3, 5)
	// Validate required fields and ranges
	if err := validateEvalOut(out); err != nil { return llmEvalOut{}, err }
	return out, nil
}

func extractFirstJSONObject(s string) (string, bool) {
	start := strings.Index(s, "{")
	if start == -1 { return "", false }
	// naive brace matching
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' { depth++ }
		if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen { return s }
	if maxLen <= 3 { return s[:maxLen] }
	return s[:maxLen-3] + "..."
}

func trimSentence(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen { return s }
	// try to cut at last period before max
	idx := strings.LastIndex(s[:maxLen], ".")
	if idx > 20 { return s[:idx+1] }
	return s[:int(math.Min(float64(len(s)), float64(maxLen)))]
}

func limitSentences(s string, minCount, maxCount int) string {
	s = strings.TrimSpace(s)
	if s == "" || maxCount <= 0 { return s }
	// naive sentence split
	parts := []string{}
	curr := strings.Builder{}
	for i := 0; i < len(s); i++ {
		curr.WriteByte(s[i])
		if s[i] == '.' || s[i] == '!' || s[i] == '?' {
			// include sentence terminator
			seg := strings.TrimSpace(curr.String())
			if seg != "" { parts = append(parts, seg) }
			curr.Reset()
			// skip trailing spaces
			for i+1 < len(s) && (s[i+1] == ' ' || s[i+1] == '\n' || s[i+1] == '\t') { i++ }
		}
	}
	// tail without terminator
	if curr.Len() > 0 {
		tail := strings.TrimSpace(curr.String())
		if tail != "" { parts = append(parts, tail) }
	}
	if len(parts) == 0 { return s }
	if len(parts) < minCount { return s }
	if len(parts) > maxCount { parts = parts[:maxCount] }
	return strings.Join(parts, " ")
}

// =========================
// LLM Chaining: Extraction
// =========================

type cvExtractOut struct {
	Skills      []string `json:"skills"`
	Experiences []string `json:"experiences"`
	Projects    []string `json:"projects"`
	Summary     string   `json:"summary"`
}

type projectExtractOut struct {
	Requirements []string `json:"requirements"`
	Architecture []string `json:"architecture"`
	Strengths    []string `json:"strengths"`
	Risks        []string `json:"risks"`
	Summary      string   `json:"summary"`
}

func buildCVExtractSystemPrompt() string {
	return strings.TrimSpace(`You are a concise information extractor. Return ONLY valid JSON with this schema and nothing else:
{
  "skills": string[] (deduplicated, lowercase terms),
  "experiences": string[] (role@company with dates if present, concise),
  "projects": string[] (concise project titles or areas),
  "summary": string (1-2 sentences)
}`)
}

func buildCVExtractUserPrompt(cvText string) string {
	b := &strings.Builder{}
	b.WriteString("Candidate CV Text (truncated):\n")
	b.WriteString(truncate(cvText, 3500))
	b.WriteString("\nReturn JSON only.")
	return b.String()
}

func buildProjectExtractSystemPrompt() string {
	return strings.TrimSpace(`You are a concise project analyzer. Return ONLY valid JSON with this schema and nothing else:
{
  "requirements": string[] (core requirements/features),
  "architecture": string[] (key components: storage, queues, services, infra),
  "strengths": string[] (technical strengths),
  "risks": string[] (gaps or risks),
  "summary": string (1-2 sentences)
}`)
}

func buildProjectExtractUserPrompt(projectText string) string {
	b := &strings.Builder{}
	b.WriteString("Project Report Text (truncated):\n")
	b.WriteString(truncate(projectText, 3500))
	b.WriteString("\nReturn JSON only.")
	return b.String()
}

func parseCVExtract(s string) (cvExtractOut, error) {
	js, ok := extractFirstJSONObject(s)
	if !ok { return cvExtractOut{}, fmt.Errorf("invalid json: not found") }
	var out cvExtractOut
	if err := json.Unmarshal([]byte(js), &out); err != nil { return cvExtractOut{}, fmt.Errorf("invalid json: %w", err) }
	// minimal sanity
	if len(out.Skills) == 0 && out.Summary == "" && len(out.Projects) == 0 && len(out.Experiences) == 0 {
		return cvExtractOut{}, fmt.Errorf("schema invalid: empty extract")
	}
	return out, nil
}

func parseProjectExtract(s string) (projectExtractOut, error) {
	js, ok := extractFirstJSONObject(s)
	if !ok { return projectExtractOut{}, fmt.Errorf("invalid json: not found") }
	var out projectExtractOut
	if err := json.Unmarshal([]byte(js), &out); err != nil { return projectExtractOut{}, fmt.Errorf("invalid json: %w", err) }
	if len(out.Requirements) == 0 && out.Summary == "" && len(out.Architecture) == 0 {
		return projectExtractOut{}, fmt.Errorf("schema invalid: empty extract")
	}
	return out, nil
}

// =========================
// LLM Chaining: Evaluation from Extracts (+RAG)
// =========================

func buildEvaluateFromExtractsUserPrompt(cvx cvExtractOut, prx projectExtractOut, jobDesc, brief string, jobCtx, rubricCtx []string) string {
	b := &strings.Builder{}
	b.WriteString("Job Description (User input):\n")
	b.WriteString(truncate(jobDesc, 2000))
	if len(jobCtx) > 0 {
		b.WriteString("\nJob Description (Retrieved context):\n")
		for i, c := range jobCtx {
			if i >= 6 { break }
			b.WriteString("- ")
			b.WriteString(truncate(c, 800))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nStudy Case Brief (User input):\n")
	b.WriteString(truncate(brief, 2000))
	if len(rubricCtx) > 0 {
		b.WriteString("\nScoring Rubric (Retrieved context):\n")
		for i, c := range rubricCtx {
			if i >= 6 { break }
			b.WriteString("- ")
			b.WriteString(truncate(c, 800))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nExtracted CV Info:\n")
	if len(cvx.Skills) > 0 { b.WriteString("skills: "); b.WriteString(strings.Join(cvx.Skills, ", ")); b.WriteString("\n") }
	if len(cvx.Experiences) > 0 { b.WriteString("experiences: "); b.WriteString(strings.Join(cvx.Experiences, "; ")); b.WriteString("\n") }
	if len(cvx.Projects) > 0 { b.WriteString("projects: "); b.WriteString(strings.Join(cvx.Projects, "; ")); b.WriteString("\n") }
	if strings.TrimSpace(cvx.Summary) != "" { b.WriteString("summary: "); b.WriteString(trimSentence(cvx.Summary, 400)); b.WriteString("\n") }

	b.WriteString("\nExtracted Project Info:\n")
	if len(prx.Requirements) > 0 { b.WriteString("requirements: "); b.WriteString(strings.Join(prx.Requirements, "; ")); b.WriteString("\n") }
	if len(prx.Architecture) > 0 { b.WriteString("architecture: "); b.WriteString(strings.Join(prx.Architecture, "; ")); b.WriteString("\n") }
	if len(prx.Strengths) > 0 { b.WriteString("strengths: "); b.WriteString(strings.Join(prx.Strengths, "; ")); b.WriteString("\n") }
	if len(prx.Risks) > 0 { b.WriteString("risks: "); b.WriteString(strings.Join(prx.Risks, "; ")); b.WriteString("\n") }
	if strings.TrimSpace(prx.Summary) != "" { b.WriteString("summary: "); b.WriteString(trimSentence(prx.Summary, 400)); b.WriteString("\n") }

	b.WriteString("\nReturn JSON only.")
	return b.String()
}
