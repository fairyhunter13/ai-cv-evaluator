package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type respErr struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func Test_writeError_Mapping(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid", domain.ErrInvalidArgument, http.StatusBadRequest, "INVALID_ARGUMENT"},
		{"notfound", domain.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"conflict", domain.ErrConflict, http.StatusConflict, "CONFLICT"},
		{"rate", domain.ErrRateLimited, http.StatusTooManyRequests, "RATE_LIMITED"},
		{"upstream_to", domain.ErrUpstreamTimeout, http.StatusServiceUnavailable, "UPSTREAM_TIMEOUT"},
		{"upstream_rl", domain.ErrUpstreamRateLimit, http.StatusServiceUnavailable, "UPSTREAM_RATE_LIMIT"},
		{"schema", domain.ErrSchemaInvalid, http.StatusServiceUnavailable, "SCHEMA_INVALID"},
		{"internal", assertError("boom"), http.StatusInternalServerError, "INTERNAL"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			rw := httptest.NewRecorder()
			writeError(rw, r, c.err, nil)
			res := rw.Result()
			if res.StatusCode != c.wantStatus {
				t.Fatalf("status: got %d want %d", res.StatusCode, c.wantStatus)
			}
			var e respErr
			_ = json.NewDecoder(res.Body).Decode(&e)
			_ = res.Body.Close()
			if e.Error.Code != c.wantCode {
				t.Fatalf("code: got %s want %s", e.Error.Code, c.wantCode)
			}
		})
	}
}

type assertError string

func (a assertError) Error() string { return string(a) }
