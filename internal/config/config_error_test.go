package config

import "testing"

func Test_Load_ErrorOnBadDuration(t *testing.T) {
	t.Setenv("HTTP_READ_TIMEOUT", "bad")
	if _, err := Load(); err == nil {
		t.Fatalf("expected error for bad duration")
	}
}
