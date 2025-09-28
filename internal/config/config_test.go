package config

import (
	"os"
	"testing"
	"github.com/stretchr/testify/require"
)

func Test_Load_And_AdminEnabled(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret")
	t.Setenv("ADMIN_SESSION_SECRET", "abcd")
	// also test chat fallbacks parsing
	t.Setenv("CHAT_FALLBACK_MODELS", "openai/gpt-4o-mini,anthropic/claude-3.5-sonnet")

	cfg, err := Load()
	if err != nil { t.Fatalf("load err: %v", err) }
	if !cfg.AdminEnabled() { t.Fatalf("expected AdminEnabled true") }
	if len(cfg.ChatFallbackModels) != 2 { t.Fatalf("fallbacks not parsed: %+v", cfg.ChatFallbackModels) }
	if !cfg.IsDev() { t.Fatalf("expected IsDev true") }
	if cfg.IsProd() { t.Fatalf("expected IsProd false") }

	// unset admin to ensure AdminEnabled false
	require.NoError(t, os.Unsetenv("ADMIN_USERNAME"))
	require.NoError(t, os.Unsetenv("ADMIN_PASSWORD"))
	require.NoError(t, os.Unsetenv("ADMIN_SESSION_SECRET"))
	cfg, err = Load()
	if err != nil { t.Fatalf("reload err: %v", err) }
	if cfg.AdminEnabled() { t.Fatalf("expected AdminEnabled false") }
}
