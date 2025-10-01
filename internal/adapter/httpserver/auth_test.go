package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
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

func Test_SessionManager_Create_Validate_And_Middleware(t *testing.T) {
	cfg := config.Config{AdminSessionSecret: "abcd", AppEnv: "dev"}
	sm := NewSessionManager(cfg)
	val, err := sm.CreateSession("admin")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	sd, err := sm.ValidateSession(val)
	if err != nil || sd == nil || sd.Username != "admin" {
		t.Fatalf("validate failed: %v %+v", err, sd)
	}

	// no cookie => redirect
	nextCalled := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { nextCalled = true })
	req := httptest.NewRequest("GET", "/admin/", nil)
	rw := httptest.NewRecorder()
	sm.AuthRequired(next).ServeHTTP(rw, req)
	if rw.Result().StatusCode != http.StatusSeeOther || nextCalled {
		t.Fatalf("expected redirect for missing cookie")
	}

	// with cookie => next called
	req2 := httptest.NewRequest("GET", "/admin/", nil)
	rw2 := httptest.NewRecorder()
	sm.SetSessionCookie(rw2, val)
	cookie := rw2.Result().Cookies()[0]
	req2.AddCookie(cookie)
	nextCalled = false
	sm.AuthRequired(next).ServeHTTP(rw2, req2)
	if !nextCalled {
		t.Fatalf("expected next to be called with valid session")
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
