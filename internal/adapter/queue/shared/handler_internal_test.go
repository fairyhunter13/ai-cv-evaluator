package shared

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

// --- Test helpers and mocks ---

func createAIClientOK(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0.1, 0.2, 0.3}}, nil)
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
		if strings.Contains(systemPrompt, "expert CV analyzer") {
			return "Key insights extracted: Go, Microservices", nil
		}
		if strings.Contains(systemPrompt, "Perform a detailed analysis") {
			return `{
				"technical_score": 4.0,
				"experience_score": 3.0,
				"achievements_score": 4.0,
				"cultural_score": 3.0,
				"correctness_score": 4.0,
				"quality_score": 4.0,
				"resilience_score": 3.0,
				"docs_score": 4.0,
				"creativity_score": 3.0,
				"cv_feedback": "Good CV",
				"project_feedback": "Good project",
				"overall_summary": "Good overall"
			}`, nil
		}
		return `{
			"cv_match_rate": 0.9,
			"cv_feedback": "Solid",
			"project_score": 9.1,
			"project_feedback": "Strong",
			"overall_summary": "Great fit"
		}`, nil
	})
	return mockClient
}

func createAIClientInvalidJSON(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0}}, nil)
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("not a json", nil)
	return mockClient
}

func createAIClientRawInvalidNormOK(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0.1}}, nil)
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
		if strings.Contains(systemPrompt, "Perform a detailed analysis") {
			return "{invalid}", nil
		}
		if strings.Contains(systemPrompt, "scoring normalization expert") {
			return `{
				"cv_match_rate": 0.8,
				"cv_feedback": "Brief",
				"project_score": 8.2,
				"project_feedback": "Brief",
				"overall_summary": "Brief"
			}`, nil
		}
		return "{}", nil
	})
	return mockClient
}

func createAIClientExtractionErr(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{{0.1}}, nil)
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ domain.Context, systemPrompt, _ string, _ int) (string, error) {
		if strings.Contains(systemPrompt, "expert CV analyzer") {
			return "", errors.New("extract fail")
		}
		return `{"cv_match_rate":0.5,"cv_feedback":"ok","project_score":5.0,"project_feedback":"ok","overall_summary":"ok"}`, nil
	})
	return mockClient
}

func createAIClientEmbedErr(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return(nil, errors.New("embed fail"))
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("{}", nil)
	return mockClient
}

func newTestQdrantOK() *qdrantcli.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{"payload": map[string]any{"text": "Context A JavaScript"}, "score": 0.9},
					{"payload": map[string]any{"text": "Context B Python"}, "score": 0.5},
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return qdrantcli.New(srv.URL, "")
}

func newTestQdrantErr() *qdrantcli.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search") {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return qdrantcli.New(srv.URL, "")
}

func createAIClientEmptyEmbed(_ *testing.T) *domainmocks.AIClient {
	mockClient := &domainmocks.AIClient{}
	mockClient.On("Embed", mock.Anything, mock.Anything).Return([][]float32{}, nil)
	mockClient.On("ChatJSON", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("{}", nil)
	return mockClient
}

func newTestQdrantEmptyResults() *qdrantcli.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return qdrantcli.New(srv.URL, "")
}

func newTestQdrantInvalidPayload() *qdrantcli.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/points/search") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": []map[string]any{
					{"payload": "invalid", "score": 0.9},
					{"payload": map[string]any{"text": ""}, "score": 0.5},
					{"payload": map[string]any{"other": "value"}, "score": 0.3},
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	return qdrantcli.New(srv.URL, "")
}

// --- Tests for helpers and branches ---

func Test_buildUserWithContext_NoRAG_NoChain(t *testing.T) {
	ai := createAIClientOK(t)
	prompt, err := buildUserWithContext(context.Background(), ai, nil, "cv text", "proj text", "job desc", "study", "scoring rubric")
	assert.NoError(t, err)
	assert.Contains(t, prompt, "You are an AI CV evaluator")
}

func Test_buildUserWithContext_WithRAG(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantOK()
	prompt, err := buildUserWithContext(context.Background(), ai, q, "cv text", "proj text", "job desc", "study", "scoring rubric")
	assert.NoError(t, err)
	// The prompt should contain the enhanced context with insights
	assert.Contains(t, prompt, "Key Insights from Analysis")
}

func Test_buildUserWithContext_WithChain_ExtractionFallbackOnErr(t *testing.T) {
	ai := createAIClientExtractionErr(t)
	q := newTestQdrantOK()
	prompt, err := buildUserWithContext(context.Background(), ai, q, "cv text", "proj text", "job desc", "study", "scoring rubric")
	assert.NoError(t, err)
	// When extraction fails, base prompt is returned
	assert.Contains(t, prompt, "You are an AI CV evaluator")
}

func Test_retrieveRAGContext_HandlesErrors(t *testing.T) {
	// Embed error -> advanced search errors -> returns empty context, nil error
	ai := createAIClientEmbedErr(t)
	q := newTestQdrantOK()
	ctx, err := retrieveRAGContext(context.Background(), ai, q, "cv", "proj")
	assert.NoError(t, err)
	assert.Equal(t, "", ctx)

	// Qdrant error path -> empty
	ai2 := createAIClientOK(t)
	q2 := newTestQdrantErr()
	ctx2, err2 := retrieveRAGContext(context.Background(), ai2, q2, "cv", "proj")
	assert.NoError(t, err2)
	assert.Equal(t, "", ctx2)
}

func Test_searchCollectionAdvanced_ReturnsTopK(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantOK()
	contexts, err := searchCollectionAdvanced(context.Background(), ai, q, "job_description", "JavaScript dev", 1)
	assert.NoError(t, err)
	assert.Len(t, contexts, 1)
	assert.Contains(t, contexts[0], "Context A")
}

func Test_searchCollection_Success(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantOK()
	contexts, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.NoError(t, err)
	assert.Len(t, contexts, 2)
	assert.Contains(t, contexts[0], "Context A")
	assert.Contains(t, contexts[1], "Context B")
}

func Test_searchCollection_EmbeddingError(t *testing.T) {
	ai := createAIClientEmbedErr(t)
	q := newTestQdrantOK()
	_, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate embeddings")
}

func Test_searchCollection_NoEmbeddings(t *testing.T) {
	ai := createAIClientEmptyEmbed(t)
	q := newTestQdrantOK()
	_, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no embeddings generated")
}

func Test_searchCollection_QdrantError(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantErr()
	_, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to search collection")
}

func Test_searchCollection_EmptyResults(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantEmptyResults()
	contexts, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.NoError(t, err)
	assert.Empty(t, contexts)
}

func Test_searchCollection_InvalidPayload(t *testing.T) {
	ai := createAIClientOK(t)
	q := newTestQdrantInvalidPayload()
	contexts, err := searchCollection(context.Background(), ai, q, "job_description", "JavaScript dev", 2)
	assert.NoError(t, err)
	assert.Empty(t, contexts)
}

func Test_performExtractionPass_FallbackOnAIError(t *testing.T) {
	ai := createAIClientExtractionErr(t)
	base := "base"
	out, err := performExtractionPass(context.Background(), ai, base, "rag")
	assert.NoError(t, err)
	assert.Equal(t, base, out)
}

func Test_performTwoPassEvaluation_Success(t *testing.T) {
	ai := createAIClientOK(t)
	user := "user prompt"
	res, err := performTwoPassEvaluation(context.Background(), ai, user, "job-1")
	assert.NoError(t, err)
	assert.InDelta(t, 0.0, res.CVMatchRate, 1.0)
	assert.InDelta(t, 1.0, res.ProjectScore, 9.0)
	assert.NotEmpty(t, res.CVFeedback)
}

func Test_performTwoPassEvaluation_FallbackNormalization(t *testing.T) {
	ai := createAIClientRawInvalidNormOK(t)
	res, err := performTwoPassEvaluation(context.Background(), ai, "user", "job-2")
	assert.NoError(t, err)
	assert.InDelta(t, 0.8, res.CVMatchRate, 0.001)
	assert.InDelta(t, 8.2, res.ProjectScore, 0.001)
}

func Test_performSinglePassEvaluation_InvalidJSON(t *testing.T) {
	ai := createAIClientInvalidJSON(t)
	_, err := performSinglePassEvaluation(context.Background(), ai, "user", "job-3")
	assert.Error(t, err)
}

func Test_parseEvaluationResponse_ClampingAndDefaults(t *testing.T) {
	resp := `{"cv_match_rate": 2.5, "cv_feedback": "", "project_score": 20, "project_feedback": "", "overall_summary": ""}`
	res, err := parseEvaluationResponse(resp, "job-4")
	assert.NoError(t, err)
	assert.Equal(t, 1.0, res.CVMatchRate)
	assert.Equal(t, 10.0, res.ProjectScore)
	assert.NotEmpty(t, res.CVFeedback)
	assert.NotEmpty(t, res.ProjectFeedback)
	assert.NotEmpty(t, res.OverallSummary)
}

func Test_validateNoCoTLeakage_Detects(t *testing.T) {
	err := validateNoCoTLeakage(`{"cv_feedback":"I think this is great"}`)
	assert.Error(t, err)
}

func Test_reRankByScore_Sorts(t *testing.T) {
	in := []SearchResult{{"a", 0.1}, {"b", 0.9}, {"c", 0.5}}
	out := reRankByScore(in)
	assert.Equal(t, "b", out[0].Text)
}

func Test_reRankByRelevance_Order(t *testing.T) {
	contexts := []string{"Go microservices", "Python", "JavaScript React"}
	cv := "JavaScript React dev"
	proj := "web app"
	out := reRankByRelevance(contexts, cv, proj)
	assert.Equal(t, 3, len(out))
}

func Test_countKeywordOverlaps_Edge(t *testing.T) {
	assert.Equal(t, 0.0, countKeywordOverlaps("", ""))
	assert.Equal(t, 0.0, countKeywordOverlaps("a b", ""))
}

func Test_calculateRAGEffectiveness_Zero(t *testing.T) {
	assert.Equal(t, 0.0, calculateRAGEffectiveness(nil, "q"))
}

func Test_calculateWeightedScores_Clamps(t *testing.T) {
	cv := map[string]float64{"technical": -10, "experience": -10, "achievements": -10, "cultural": -10}
	proj := map[string]float64{"correctness": -10, "quality": -10, "resilience": -10, "docs": -10, "creativity": -10}
	cvW, projW := calculateWeightedScores(cv, proj)
	assert.Equal(t, 0.0, cvW)
	assert.Equal(t, 1.0, projW)
}
