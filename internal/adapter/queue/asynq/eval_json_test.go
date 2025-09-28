package asynqadp

import (
	"encoding/json"
	"strings"
	"testing"
)

func Test_extractFirstJSONObject(t *testing.T) {
	js := "prefix {\"a\":1, \"b\":2} suffix"
	out, ok := extractFirstJSONObject(js)
	if !ok { t.Fatalf("expected ok") }
	var m map[string]int
	if err := json.Unmarshal([]byte(out), &m); err != nil { t.Fatalf("json err: %v", err) }
	if m["a"] != 1 || m["b"] != 2 { t.Fatalf("unexpected: %#v", m) }
}

func Test_truncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" { t.Fatalf("got %q", got) }
	if got := truncate("helloworld", 5); got != "he..." { t.Fatalf("got %q", got) }
	if got := truncate("abcdef", 3); got != "abc" { t.Fatalf("got %q", got) }
}

func Test_trimSentence(t *testing.T) {
	// should cut at last period before max
	in := "This is a long sentence. Another one. And another that is very long without end markers"
	got := trimSentence(in, 40)
	if !strings.HasSuffix(got, ".") { t.Fatalf("expected cut at period, got %q", got) }
}

func Test_limitSentences(t *testing.T) {
	in := "One. Two! Three? Four. Five. Six."
	got := limitSentences(in, 1, 3)
	// ensure at most 3 terminal sentences
	cnt := strings.Count(got, ".") + strings.Count(got, "?") + strings.Count(got, "!")
	if cnt > 3 { t.Fatalf("too many sentences: %d in %q", cnt, got) }
}

func Test_parseAndNormalize_ValidClampAndGuards(t *testing.T) {
	in := `Some header text {"cv_match_rate": 1.5, "cv_feedback": "Good.", "project_score": 12, "project_feedback": "Nice.", "overall_summary": "One. Two. Three. Six. Seven."} trailing`
	out, err := parseAndNormalize(in)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if out.CVMatchRate > 1.0 || out.CVMatchRate < 0.0 { t.Fatalf("cv_match_rate not clamped: %v", out.CVMatchRate) }
	if out.ProjectScore > 10.0 || out.ProjectScore < 1.0 { t.Fatalf("project_score not clamped: %v", out.ProjectScore) }
	// overall summary must be 3-5 sentences; ensure it is not empty
	if strings.TrimSpace(out.OverallSummary) == "" { t.Fatalf("summary empty") }
}

func Test_parseAndNormalize_CoTRejected(t *testing.T) {
	in := `{"cv_match_rate": 0.5, "cv_feedback": "Good.", "project_score": 5, "project_feedback": "Nice.", "overall_summary": "Step 1: think."}`
	_, err := parseAndNormalize(in)
	if err == nil || !strings.Contains(err.Error(), "chain-of-thought") {
		t.Fatalf("expected CoT rejection, got %v", err)
	}
}

func Test_buildSystemAndNormalizationPrompts(t *testing.T) {
	sp := buildSystemPrompt()
	if !strings.Contains(sp, "cv_match_rate") || !strings.Contains(sp, "Return ONLY valid JSON") {
		t.Fatalf("system prompt missing constraints: %q", sp)
	}
	nsp := buildNormalizationSystemPrompt()
	if !strings.Contains(nsp, "Return ONLY valid JSON") { t.Fatalf("normalization prompt missing guard") }
	unp := buildNormalizationUserPrompt(llmEvalOut{CVMatchRate: 1, ProjectScore: 10})
	if !strings.Contains(unp, "normalize this JSON") { t.Fatalf("unexpected norm user prompt: %q", unp) }
}

func Test_buildPrompts_RAG_And_FromExtracts(t *testing.T) {
	up := buildUserPrompt("cv", "proj", strings.Repeat("j", 5000), strings.Repeat("b", 5000))
	if !strings.Contains(up, "Return JSON only.") { t.Fatalf("missing guard") }
	// RAG prompt includes context markers
	rag := buildUserPromptRAG("cv", "proj", "jobdesc", "brief", []string{"ctx1", "ctx2"}, []string{"rub1"})
	if !strings.Contains(rag, "Retrieved context") { t.Fatalf("missing RAG context sections") }
	pe := buildEvaluateFromExtractsUserPrompt(cvExtractOut{Skills: []string{"go", "aws"}, Summary: "ok."}, projectExtractOut{Requirements: []string{"req1"}}, "jobdesc", "brief", []string{"ctx1"}, []string{"rub1"})
	if !strings.Contains(pe, "Extracted CV Info") || !strings.Contains(pe, "Extracted Project Info") { t.Fatalf("missing extract sections") }
}
