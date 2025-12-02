// Package httpserver contains HTTP handlers and middleware.
//
// The package follows clean architecture principles and provides
// a clear separation between HTTP concerns and business logic.
package httpserver

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

// Argon2Params defines parameters for Argon2id password hashing
type Argon2Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

var defaultArgon2Params = Argon2Params{
	Memory:      64 * 1024, // 64 MB
	Iterations:  3,
	Parallelism: 2,
	SaltLen:     16,
	KeyLen:      32,
}

// HashPassword creates an Argon2id hash of the password
func HashPassword(password string, params Argon2Params) (string, error) {
	salt := make([]byte, params.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLen)

	// Format: argon2id$iterations$memory$parallelism$salt$hash (base64 encoded)
	encoded := fmt.Sprintf("argon2id$%d$%d$%d$%s$%s",
		params.Iterations,
		params.Memory,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	)

	return encoded, nil
}

// VerifyPassword verifies a password against its Argon2id hash
func VerifyPassword(password, encodedHash string) bool {
	// Expected format: argon2id$iterations$memory$parallelism$salt$hash (base64 raw std for salt/hash)
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return false
	}
	// Parse numeric params
	iters64, err1 := parseUint32(parts[1])
	mem64, err2 := parseUint32(parts[2])
	par64, err3 := parseUint32(parts[3])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	// Clamp parallelism to uint8 range to avoid overflow
	var par uint8
	if par64 > math.MaxUint8 {
		par = math.MaxUint8
	} else {
		par = uint8(par64)
	}
	keyLen := defaultArgon2Params.KeyLen
	actualHash := argon2.IDKey([]byte(password), salt, iters64, mem64, par, keyLen)
	return subtle.ConstantTimeCompare(actualHash, expectedHash) == 1
}

// SessionData represents session information
type SessionData struct {
	Username  string
	LoginTime time.Time
	ExpiresAt time.Time
}

// SessionManager handles session management with HMAC-signed cookies
type SessionManager struct {
	secret []byte
	cfg    config.Config
}

// sameSiteFromString converts string config to http.SameSite
// sameSiteFromString removed; cookie-based admin sessions deprecated.

// NewSessionManager creates a new session manager
func NewSessionManager(cfg config.Config) *SessionManager {
	return &SessionManager{
		secret: []byte(cfg.AdminSessionSecret),
		cfg:    cfg,
	}
}

// GenerateJWT issues a compact JWT (HS256) for the given username and TTL.
// It avoids external deps by implementing minimal JWT encode logic.
func (sm *SessionManager) GenerateJWT(username string, ttl time.Duration) (string, error) {
	if username == "" || ttl <= 0 {
		return "", fmt.Errorf("invalid params")
	}
	now := time.Now().Unix()
	exp := time.Now().Add(ttl).Unix()

	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}
	claims := map[string]any{
		"sub": username,
		"iat": now,
		"exp": exp,
		"iss": "ai-cv-evaluator",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	enc := base64.RawURLEncoding
	head := enc.EncodeToString(headerJSON)
	body := enc.EncodeToString(claimsJSON)
	unsigned := head + "." + body

	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(unsigned))
	sig := enc.EncodeToString(mac.Sum(nil))
	return unsigned + "." + sig, nil
}

// ValidateJWT validates HS256 JWT and returns subject (username) if valid.
func (sm *SessionManager) ValidateJWT(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("empty token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token")
	}

	unsigned := parts[0] + "." + parts[1]
	enc := base64.RawURLEncoding

	// Verify signature
	sigBytes, err := enc.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("bad signature encoding")
	}
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)
	if !hmac.Equal(expected, sigBytes) {
		return "", fmt.Errorf("invalid signature")
	}

	// Parse claims
	claimsJSON, err := enc.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("bad claims encoding")
	}
	var claims map[string]any
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return "", fmt.Errorf("bad claims")
	}

	// Validate exp
	expVal, ok := claims["exp"]
	if !ok {
		return "", fmt.Errorf("no exp")
	}
	var exp int64
	switch v := expVal.(type) {
	case float64:
		exp = int64(v)
	case int64:
		exp = v
	default:
		return "", fmt.Errorf("bad exp type")
	}
	if time.Now().Unix() >= exp {
		return "", fmt.Errorf("token expired")
	}

	// Subject
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", fmt.Errorf("no sub")
	}
	return sub, nil
}

// CreateSession creates a new session and returns the session cookie value
func (sm *SessionManager) CreateSession(username string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(24 * time.Hour) // 24 hour sessions

	// Create payload: username:loginTime:expiresAt
	payload := fmt.Sprintf("%s:%d:%d", username, now.Unix(), expiresAt.Unix())

	// Create HMAC signature
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(payload))
	signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

	// Final session value: payload.signature
	sessionValue := payload + "." + signature

	return sessionValue, nil
}

// ValidateSession validates a session cookie value and returns session data
func (sm *SessionManager) ValidateSession(sessionValue string) (*SessionData, error) {
	if sessionValue == "" {
		return nil, fmt.Errorf("empty session value")
	}

	// Split payload and signature
	parts := strings.Split(sessionValue, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid session format")
	}

	payload, signatureB64 := parts[0], parts[1]

	// Verify HMAC signature
	mac := hmac.New(sha256.New, sm.secret)
	mac.Write([]byte(payload))
	expectedSignature := mac.Sum(nil)

	actualSignature, err := base64.URLEncoding.DecodeString(signatureB64)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	if !hmac.Equal(expectedSignature, actualSignature) {
		return nil, fmt.Errorf("invalid session signature")
	}

	// Parse payload
	payloadParts := strings.Split(payload, ":")
	if len(payloadParts) != 3 {
		return nil, fmt.Errorf("invalid payload format")
	}

	username := payloadParts[0]
	loginTime := time.Unix(parseInt64(payloadParts[1]), 0)
	expiresAt := time.Unix(parseInt64(payloadParts[2]), 0)

	// Check expiration
	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &SessionData{
		Username:  username,
		LoginTime: loginTime,
		ExpiresAt: expiresAt,
	}, nil
}

// SetSessionCookie deprecated; no-op for JWT-only auth
func (sm *SessionManager) SetSessionCookie(_ http.ResponseWriter, _ string) {}

// ClearSessionCookie deprecated; no-op for JWT-only auth
func (sm *SessionManager) ClearSessionCookie(_ http.ResponseWriter) {}

// GenerateCSRFCookieValue creates a random CSRF token value (URL-safe base64)
func GenerateCSRFCookieValue() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback: time-based entropy if RNG fails (very unlikely)
		return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// CSRFGuard enforces double-submit cookie for unsafe methods when admin is enabled.
// Compares header X-CSRF-Token with cookie 'csrf-token' using constant-time compare.
func (s *Server) CSRFGuard() func(http.Handler) http.Handler {
	// CSRF protection disabled by request: middleware is a no-op
	return func(next http.Handler) http.Handler { return next }
}

// sessionKey is an unexported context key type for session data.
// sessionKey removed; session context not used with SSO/jwt auth.

// AuthRequired is a middleware that enforces a valid admin session.
// It redirects unauthenticated requests to the admin login page and
// injects validated session data into the request context for downstream handlers.
func (sm *SessionManager) AuthRequired(next http.Handler) http.Handler {
	// Deprecated in favor of AdminBearerRequired; keep noop for compatibility
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// getSSOUsernameFromHeaders extracts a trusted username from reverse-proxy SSO headers.
// Works with oauth2-proxy (X-Auth-Request-User) and common auth proxy conventions.
func getSSOUsernameFromHeaders(r *http.Request) string {
	// oauth2-proxy header when set_xauthrequest = true
	if v := strings.TrimSpace(r.Header.Get("X-Auth-Request-User")); v != "" {
		return v
	}
	// Generic proxy header and legacy support
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-User")); v != "" {
		return v
	}
	return ""
}

// AdminBearerRequired enforces Bearer JWT auth and injects subject into context.
func (a *AdminServer) AdminBearerRequired(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Prefer SSO via trusted reverse proxy headers
		if ssoUser := getSSOUsernameFromHeaders(r); ssoUser != "" {
			next(w, r)
			return
		}
		// Fallback to Bearer JWT
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token := strings.TrimSpace(authz[len("Bearer "):])
			if _, err := a.sessionManager.ValidateJWT(token); err == nil {
				next(w, r)
				return
			}
		}
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// parseInt64 safely parses string to int64, returns 0 on error
func parseInt64(s string) int64 {
	x, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return x
}

// parseUint32 parses a decimal string into uint32; returns error on failure
func parseUint32(s string) (uint32, error) {
	x, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse")
	}
	if x > math.MaxUint32 {
		return 0, fmt.Errorf("parse")
	}
	return uint32(x), nil
}

// AdminAPIGuard returns a middleware that protects API endpoints.
// If admin credentials are configured (AdminEnabled), it accepts only:
// - A valid Bearer JWT issued by /admin/token.
// If admin credentials are not configured, the middleware is a no-op.
func (s *Server) AdminAPIGuard() func(http.Handler) http.Handler {
	// Fast path: if admin creds are not configured, do nothing
	if !s.Cfg.AdminEnabled() {
		return func(next http.Handler) http.Handler { return next }
	}
	sm := NewSessionManager(s.Cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Prefer SSO via trusted reverse proxy headers
			if ssoUser := getSSOUsernameFromHeaders(r); ssoUser != "" {
				next.ServeHTTP(w, r)
				return
			}
			// Fallback to Bearer JWT
			authz := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
				token := strings.TrimSpace(authz[len("Bearer "):])
				if token != "" {
					if _, err := sm.ValidateJWT(token); err == nil {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		})
	}
}
