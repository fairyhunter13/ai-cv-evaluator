//go:build ignore
// This integration test is intentionally disabled. Integration tests are no longer used.

package usecase_test

import (
    "testing"
)

// Placeholder integration test. In CI, this can be expanded to use testcontainers-go
// to launch Postgres, Redis, Qdrant, and Tika, then verify end-to-end flows.
func TestIntegration_Placeholder(t *testing.T) {
    // TODO: implement with testcontainers
    t.Skip("integration tests pending full testcontainers setup")
}
