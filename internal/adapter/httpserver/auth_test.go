package httpserver

import "testing"

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
