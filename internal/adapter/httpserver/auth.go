package httpserver

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
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
    if par64 > math.MaxUint8 { par = math.MaxUint8 } else { par = uint8(par64) }
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

// NewSessionManager creates a new session manager
func NewSessionManager(cfg config.Config) *SessionManager {
	return &SessionManager{
		secret: []byte(cfg.AdminSessionSecret),
		cfg:    cfg,
	}
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

// SetSessionCookie sets the session cookie on the response
func (sm *SessionManager) SetSessionCookie(w http.ResponseWriter, sessionValue string) {
	cookie := &http.Cookie{
		Name:     "session",
		Value:    sessionValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   sm.cfg.AppEnv != "dev",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	}
	http.SetCookie(w, cookie)
}

// ClearSessionCookie clears the session cookie
func (sm *SessionManager) ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   sm.cfg.AppEnv != "dev",
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
	http.SetCookie(w, cookie)
}

// sessionKey is an unexported context key type for session data.
type sessionKey struct{}

// AuthRequired middleware for protecting admin routes
func (sm *SessionManager) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil || cookie.Value == "" {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		sessionData, err := sm.ValidateSession(cookie.Value)
		if err != nil {
			sm.ClearSessionCookie(w)
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		// Add session data to request context using a typed key
		ctx := context.WithValue(r.Context(), sessionKey{}, sessionData)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
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
