package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func Test_HashPassword_VerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret", defaultArgon2Params)
	if err != nil {
		t.Fatalf("hash err: %v", err)
	}
	if !VerifyPassword("s3cret", hash) {
		t.Fatalf("verify failed")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatalf("verify should fail for wrong password")
	}
}

func Test_parseInt64(t *testing.T) {
	if parseInt64("123") != 123 {
		t.Fatalf("parse 123")
	}
	if parseInt64("x") != 0 {
		t.Fatalf("parse invalid should be 0")
	}
}

func Test_parseUint32(t *testing.T) {
	tests := []struct {
		input     string
		expected  uint32
		expectErr bool
	}{
		{"0", 0, false},
		{"1", 1, false},
		{"123", 123, false},
		{"4294967295", 4294967295, false}, // max uint32
		{"", 0, true},
		{"invalid", 0, true},
		{"-1", 0, true},
	}

	for _, tt := range tests {
		result, err := parseUint32(tt.input)
		if tt.expectErr {
			if err == nil {
				t.Errorf("parseUint32(%q) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("parseUint32(%q) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("parseUint32(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		}
	}
}

func Test_SessionManager_SetSessionCookie(_ *testing.T) {
	sm := &SessionManager{}
	w := httptest.NewRecorder()
	// Should be a no-op, just ensure it doesn't panic
	sm.SetSessionCookie(w, "test-session-id")
	// No cookie should be set since it's a no-op
}

func Test_SessionManager_ClearSessionCookie(_ *testing.T) {
	sm := &SessionManager{}
	w := httptest.NewRecorder()
	// Should be a no-op, just ensure it doesn't panic
	sm.ClearSessionCookie(w)
	// No cookie should be set since it's a no-op
}

func Test_GenerateCSRFCookieValue(t *testing.T) {
	// Generate multiple values and ensure they're unique
	values := make(map[string]bool)
	for i := 0; i < 10; i++ {
		v := GenerateCSRFCookieValue()
		if v == "" {
			t.Fatal("GenerateCSRFCookieValue returned empty string")
		}
		if values[v] {
			t.Fatalf("GenerateCSRFCookieValue returned duplicate value: %s", v)
		}
		values[v] = true
	}
}

func Test_Server_CSRFGuard(t *testing.T) {
	s := &Server{}
	middleware := s.CSRFGuard()

	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with CSRF guard
	wrapped := middleware(testHandler)

	// Make a request
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// CSRF guard is a no-op, so request should pass through
	if w.Code != http.StatusOK {
		t.Errorf("CSRFGuard blocked request, got status %d", w.Code)
	}
}

func Test_VerifyPassword_InvalidFormats(t *testing.T) {
	tests := []struct {
		name     string
		hash     string
		expected bool
	}{
		{"empty_hash", "", false},
		{"wrong_prefix", "bcrypt$1$2$3$4$5", false},
		{"too_few_parts", "argon2id$1$2$3$4", false},
		{"too_many_parts", "argon2id$1$2$3$4$5$6", false},
		{"invalid_iterations", "argon2id$invalid$65536$4$c2FsdA$aGFzaA", false},
		{"invalid_memory", "argon2id$3$invalid$4$c2FsdA$aGFzaA", false},
		{"invalid_parallelism", "argon2id$3$65536$invalid$c2FsdA$aGFzaA", false},
		{"invalid_salt_encoding", "argon2id$3$65536$4$!!!invalid!!!$aGFzaA", false},
		{"invalid_hash_encoding", "argon2id$3$65536$4$c2FsdA$!!!invalid!!!", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyPassword("password", tt.hash)
			if result != tt.expected {
				t.Errorf("VerifyPassword with %s = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func Test_SessionManager_JWT_AllPaths(t *testing.T) {
	sm := &SessionManager{secret: []byte("test-secret")}

	// Test GenerateJWT with 1 hour TTL
	token, err := sm.GenerateJWT("testuser", time.Hour)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if token == "" {
		t.Fatal("GenerateJWT returned empty token")
	}

	// Test ValidateJWT with valid token
	username, err := sm.ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}
	if username != "testuser" {
		t.Errorf("ValidateJWT returned wrong username: %s", username)
	}

	// Test ValidateJWT with empty token
	_, err = sm.ValidateJWT("")
	if err == nil {
		t.Error("ValidateJWT should fail with empty token")
	}

	// Test ValidateJWT with invalid format
	_, err = sm.ValidateJWT("invalid")
	if err == nil {
		t.Error("ValidateJWT should fail with invalid format")
	}

	// Test ValidateJWT with wrong signature
	_, err = sm.ValidateJWT(token[:len(token)-5] + "xxxxx")
	if err == nil {
		t.Error("ValidateJWT should fail with wrong signature")
	}

	// Test GenerateJWT with invalid params
	_, err = sm.GenerateJWT("", time.Hour)
	if err == nil {
		t.Error("GenerateJWT should fail with empty username")
	}

	_, err = sm.GenerateJWT("testuser", 0)
	if err == nil {
		t.Error("GenerateJWT should fail with zero TTL")
	}

	_, err = sm.GenerateJWT("testuser", -time.Hour)
	if err == nil {
		t.Error("GenerateJWT should fail with negative TTL")
	}
}

func Test_SessionManager_ValidateJWT_EdgeCases(t *testing.T) {
	sm := &SessionManager{secret: []byte("test-secret")}

	tests := []struct {
		name      string
		token     string
		expectErr bool
	}{
		{"empty_token", "", true},
		{"one_part", "abc", true},
		{"two_parts", "abc.def", true},
		{"four_parts", "abc.def.ghi.jkl", true},
		{"invalid_signature_encoding", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.!!!invalid!!!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.ValidateJWT(tt.token)
			if tt.expectErr && err == nil {
				t.Errorf("ValidateJWT(%s) should return error", tt.name)
			}
		})
	}
}
