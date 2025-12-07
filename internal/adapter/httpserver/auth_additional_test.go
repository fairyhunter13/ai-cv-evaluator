package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
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
	h := guard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
	h := admin.AdminBearerRequired(func(w http.ResponseWriter, _ *http.Request) {
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
	h := admin.AdminBearerRequired(func(w http.ResponseWriter, _ *http.Request) {
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

	h := admin.AdminBearerRequired(func(w http.ResponseWriter, _ *http.Request) {
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
	h := sm.AuthRequired(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}

func TestValidateJWT_EmptyToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	_, err := sm.ValidateJWT("")
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty token")
}

func TestValidateJWT_InvalidParts(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	_, err := sm.ValidateJWT("invalid.token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid token")
}

func TestValidateJWT_BadSignatureEncoding(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	// Token with invalid base64 in signature
	_, err := sm.ValidateJWT("header.payload.!!!invalid!!!")
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad signature encoding")
}

func TestValidateJWT_InvalidSignature(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	// Generate a valid token then modify the signature
	token, err := sm.GenerateJWT("testuser", time.Hour)
	require.NoError(t, err)

	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)

	// Modify the signature
	modifiedToken := parts[0] + "." + parts[1] + ".wrongsignature"
	_, err = sm.ValidateJWT(modifiedToken)
	require.Error(t, err)
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	// Generate a token with very short duration
	token, err := sm.GenerateJWT("testuser", 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = sm.ValidateJWT(token)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token expired")
}

func TestValidateJWT_ValidToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)
	token, err := sm.GenerateJWT("testuser", time.Hour)
	require.NoError(t, err)

	sub, err := sm.ValidateJWT(token)
	require.NoError(t, err)
	require.Equal(t, "testuser", sub)
}

func TestSetSessionCookie_NoOp(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	rec := httptest.NewRecorder()
	sm.SetSessionCookie(rec, "test-value")

	// Should be a no-op, no cookies set
	cookies := rec.Result().Cookies()
	require.Empty(t, cookies)
}

func TestClearSessionCookie_NoOp(t *testing.T) {
	t.Parallel()

	cfg := config.Config{AdminSessionSecret: "secret"}
	sm := NewSessionManager(cfg)

	rec := httptest.NewRecorder()
	sm.ClearSessionCookie(rec)

	// Should be a no-op, no cookies set
	cookies := rec.Result().Cookies()
	require.Empty(t, cookies)
}

func TestGenerateCSRFCookieValue_Length(t *testing.T) {
	t.Parallel()

	value := GenerateCSRFCookieValue()
	// 32 bytes in base64 raw URL encoding = 43 characters
	require.GreaterOrEqual(t, len(value), 40)
}
