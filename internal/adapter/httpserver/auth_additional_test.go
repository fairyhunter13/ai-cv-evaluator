package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func TestSessionManager_CreateAndValidateSession_Success(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	val, err := sm.CreateSession("admin")
	require.NoError(t, err)
	require.NotEmpty(t, val)

	sd, err := sm.ValidateSession(val)
	require.NoError(t, err)
	require.Equal(t, "admin", sd.Username)
	require.True(t, sd.ExpiresAt.After(time.Now()))
}

func TestSessionManager_ValidateSession_InvalidSignature(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	val, err := sm.CreateSession("admin")
	require.NoError(t, err)

	// Corrupt the signature part
	parts := []byte(val)
	if len(parts) > 0 {
		parts[len(parts)-1] ^= 0xFF
	}
	_, err = sm.ValidateSession(string(parts))
	require.Error(t, err)
}

func TestSessionManager_ValidateSession_Expired(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	// Manually craft an already expired payload and sign it with the same secret
	payload := "admin:1:2" // loginTime=1, expiresAt=2 (unix seconds)
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(payload))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	val := payload + "." + sig

	_, err := sm.ValidateSession(val)
	require.Error(t, err)
}

func TestGenerateCSRFCookieValue_UniqueAndNonEmpty(t *testing.T) {
	v1 := GenerateCSRFCookieValue()
	v2 := GenerateCSRFCookieValue()
	require.NotEmpty(t, v1)
	require.NotEmpty(t, v2)
	// Extremely unlikely to collide; this also catches deterministic bugs
	require.NotEqual(t, v1, v2)
}

func TestCSRFGuard_NoOpMiddleware(t *testing.T) {
	s := &Server{}
	guard := s.CSRFGuard()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/api/test", nil)

	called := false
	h := guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	h.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusNoContent, rec.Result().StatusCode)
}

func TestAdminBearerRequired_AllowsSSOHeader(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	server := &Server{Cfg: cfg}
	admin, err := NewAdminServer(cfg, server)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/protected", nil)
	req.Header.Set("X-Auth-Request-User", "alice")

	called := false
	h := admin.AdminBearerRequired(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}

func TestAdminBearerRequired_AllowsValidJWT(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	server := &Server{Cfg: cfg}
	admin, err := NewAdminServer(cfg, server)
	require.NoError(t, err)

	tok, err := admin.sessionManager.GenerateJWT("admin", time.Hour)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tok)

	called := false
	h := admin.AdminBearerRequired(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	h(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}

func TestAdminBearerRequired_UnauthorizedWithoutAuth(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	server := &Server{Cfg: cfg}
	admin, err := NewAdminServer(cfg, server)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/protected", nil)

	h := admin.AdminBearerRequired(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Result().StatusCode)
}

func TestSessionManager_SetAndClearSessionCookie_NoOp(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	rec := httptest.NewRecorder()
	sm.SetSessionCookie(rec, "value")
	sm.ClearSessionCookie(rec)

	// Deprecated methods are no-ops; they should not set any cookies
	resp := rec.Result()
	require.Empty(t, resp.Cookies())
}

func TestSessionManager_AuthRequired_PassesThrough(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/api/protected", nil)

	called := false
	h := sm.AuthRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}
