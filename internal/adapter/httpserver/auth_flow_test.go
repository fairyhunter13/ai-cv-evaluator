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

func TestAdminLoginHandler_ValidCredentials(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	req.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	w := httptest.NewRecorder()

	adminServer.AdminLoginHandler()(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Login successful")

	// Check that session cookie is set
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "session", cookies[0].Name)
	require.NotEmpty(t, cookies[0].Value)
}

func TestAdminLoginHandler_InvalidCredentials(t *testing.T) {
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
		req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
		req.Form = map[string][]string{
			"username": {tc.username},
			"password": {tc.password},
		}
		w := httptest.NewRecorder()

		adminServer.AdminLoginHandler()(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.Contains(t, w.Body.String(), "Invalid credentials")
	}
}

func TestAdminLogoutHandler(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/admin/logout", nil)
	w := httptest.NewRecorder()

	adminServer.AdminLogoutHandler()(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "Logout successful")

	// Check that session cookie is cleared
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, "session", cookies[0].Name)
	require.Empty(t, cookies[0].Value)
}

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

	// First login to get session
	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	loginReq.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	loginW := httptest.NewRecorder()
	adminServer.AdminLoginHandler()(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	// Extract session cookie
	cookies := loginW.Result().Cookies()
	require.Len(t, cookies, 1)

	// Now test status endpoint with session
	req := httptest.NewRequest(http.MethodGet, "/admin/api/status", nil)
	req.AddCookie(cookies[0])
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

func TestAdminAPIGuard_WithValidSession(t *testing.T) {
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

	// First login to get session
	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	loginReq.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	loginW := httptest.NewRecorder()
	adminServer.AdminLoginHandler()(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	// Extract session cookie
	cookies := loginW.Result().Cookies()
	require.Len(t, cookies, 1)

	// Test with valid session
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(cookies[0])
	w := httptest.NewRecorder()

	guard(testHandler).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}

func TestAdminAPIGuard_WithBasicAuth(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}

	guard := server.AdminAPIGuard()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	// Test with valid basic auth
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.SetBasicAuth("admin", "password")
	w := httptest.NewRecorder()

	guard(testHandler).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "success")
}
