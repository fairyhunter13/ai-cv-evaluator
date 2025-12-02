package real

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestFetchGroqModelsFromAPI_SuccessFiltersKnownModels(t *testing.T) {
	t.Parallel()

	// Local HTTP server returning a mix of known and unknown Groq model IDs.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "llama-3.1-8b-instant"},        // present in groqModelLimits
				{"id": "some-unknown-model-not-used"}, // filtered out
			},
		})
	}))
	defer ts.Close()

	c := &Client{
		cfg: config.Config{
			GroqBaseURL: ts.URL,
		},
		chatHC: ts.Client(),
	}

	ctx := context.Background()
	models, err := c.fetchGroqModelsFromAPI(ctx, "  test-key  ")
	require.NoError(t, err)
	require.Equal(t, []string{"llama-3.1-8b-instant"}, models)
}

func TestFetchGroqModelsFromAPI_Non200Status(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer ts.Close()

	c := &Client{
		cfg:    config.Config{GroqBaseURL: ts.URL},
		chatHC: ts.Client(),
	}

	ctx := context.Background()
	models, err := c.fetchGroqModelsFromAPI(ctx, "key")
	require.Error(t, err)
	require.Nil(t, models)
}

func TestGetGroqModels_CachesResults(t *testing.T) {
	t.Parallel()

	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "llama-3.1-8b-instant"},
			},
		})
	}))
	defer ts.Close()

	c := &Client{
		cfg: config.Config{
			GroqBaseURL:       ts.URL,
			FreeModelsRefresh: time.Hour,
		},
		chatHC: ts.Client(),
	}

	ctx := context.Background()

	first := c.getGroqModels(ctx, "g-key")
	require.NotEmpty(t, first)
	require.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "expected exactly one HTTP request on first call")

	second := c.getGroqModels(ctx, "g-key")
	require.Equal(t, first, second, "expected cached models to be returned on second call")
	require.Equal(t, int32(1), atomic.LoadInt32(&requestCount), "expected no additional HTTP requests due to caching")
}
