package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
