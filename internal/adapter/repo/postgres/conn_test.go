package postgres

import (
    "context"
    "testing"
)

func TestNewPool_InvalidDSN(t *testing.T) {
    if _, err := NewPool(context.Background(), "://bad"); err == nil {
        t.Fatalf("expected error for invalid dsn")
    }
}
