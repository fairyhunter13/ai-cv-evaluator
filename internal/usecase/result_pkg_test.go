package usecase

import (
	"errors"
	"fmt"
	"testing"
)

func Test_makeETag_ChangesWithContent(t *testing.T) {
	etag1 := makeETag(map[string]any{"a": 1})
	etag2 := makeETag(map[string]any{"a": 2})
	if etag1 == etag2 || etag1 == "" || etag2 == "" { t.Fatalf("etag not varying: %s %s", etag1, etag2) }
}

func Test_errWrapped(t *testing.T) {
	base := errors.New("boom")
	wrapped := fmt.Errorf("wrap: %w", base)
	if !errWrapped(wrapped, base) { t.Fatalf("expected wrapped to match base") }
}

func Test_errorCodeFromJobError(t *testing.T) {
	cases := map[string]string{
		"schema invalid": "SCHEMA_INVALID",
		"invalid json": "SCHEMA_INVALID",
		"rate limit": "UPSTREAM_RATE_LIMIT",
		"timeout": "UPSTREAM_TIMEOUT",
		"deadline exceeded": "UPSTREAM_TIMEOUT",
		"not found": "NOT_FOUND",
		"invalid argument": "INVALID_ARGUMENT",
		"weird": "INTERNAL",
	}
	for in, want := range cases {
		if got := errorCodeFromJobError(in); got != want { t.Fatalf("%q => %q (got %q)", in, want, got) }
	}
}
