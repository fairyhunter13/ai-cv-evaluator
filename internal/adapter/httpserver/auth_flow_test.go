package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestAdminTokenHandler_IssuesJWT(t *testing.T) {
    cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
    server := &httpserver.Server{Cfg: cfg}
    adminServer, err := httpserver.NewAdminServer(cfg, server)
    require.NoError(t, err)

    req := httptest.NewRequest(http.MethodPost, "/admin/token", nil)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Form = map[string][]string{
        "username": {"admin"},
        "password": {"password"},
    }
    w := httptest.NewRecorder()

    adminServer.AdminTokenHandler()(w, req)

    require.Equal(t, http.StatusOK, w.Code)
    var body map[string]any
    require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
    require.NotEmpty(t, body["token"])
}

func TestAdminTokenHandler_InvalidCredentials(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	testCases := []struct {
		username string
		password string
	}{
		{"wrong", "password"},
		{"admin", "wrong"},
		{"", "password"},
		{"admin", ""},
	}

	for _, tc := range testCases {
        req := httptest.NewRequest(http.MethodPost, "/admin/token", nil)
		req.Form = map[string][]string{
			"username": {tc.username},
			"password": {tc.password},
		}
		w := httptest.NewRecorder()

        adminServer.AdminTokenHandler()(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
        require.Contains(t, w.Body.String(), "Invalid credentials")
	}
}

// Logout not applicable for JWT; client discards token

func TestAdminStatusHandler_Unauthorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/status", nil)
	w := httptest.NewRecorder()

	adminServer.AdminStatusHandler()(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminStatusHandler_Authorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

    // Get JWT
    tokenReq := httptest.NewRequest(http.MethodPost, "/admin/token", nil)
    tokenReq.Form = map[string][]string{
        "username": {"admin"},
        "password": {"password"},
    }
    tokenW := httptest.NewRecorder()
    adminServer.AdminTokenHandler()(tokenW, tokenReq)
    require.Equal(t, http.StatusOK, tokenW.Code)
    var tb map[string]any
    require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &tb))
    tok := tb["token"].(string)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/status", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()

	adminServer.AdminStatusHandler()(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "authenticated", response["status"])
	require.Equal(t, "admin", response["username"])
}

func TestAdminStatusHandler_InvalidSession(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/status", nil)
	// Add invalid session cookie
	req.AddCookie(&http.Cookie{Name: "session", Value: "invalid-session"})
	w := httptest.NewRecorder()

	adminServer.AdminStatusHandler()(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminAPIGuard_NoCredentials(t *testing.T) {
	cfg := config.Config{} // No admin credentials configured
	server := &httpserver.Server{Cfg: cfg}

	guard := server.AdminAPIGuard()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	guard(testHandler).ServeHTTP(w, req)

	// Should pass through since no admin credentials are configured
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestAdminAPIGuard_WithCredentials(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}

	guard := server.AdminAPIGuard()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Test without authentication
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	guard(testHandler).ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminAPIGuard_WithBearer(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	guard := server.AdminAPIGuard()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

    // Get JWT
    tokenReq := httptest.NewRequest(http.MethodPost, "/admin/token", nil)
    tokenReq.Form = map[string][]string{
        "username": {"admin"},
        "password": {"password"},
    }
    tokenW := httptest.NewRecorder()
    adminServer.AdminTokenHandler()(tokenW, tokenReq)
    require.Equal(t, http.StatusOK, tokenW.Code)
    var tb map[string]any
    require.NoError(t, json.Unmarshal(tokenW.Body.Bytes(), &tb))
    tok := tb["token"].(string)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
    req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()

	guard(testHandler).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

// Basic Auth removed
