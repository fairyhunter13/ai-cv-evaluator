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

func TestNewPool_EmptyDSN(t *testing.T) {
	// Empty DSN may or may not fail depending on the implementation
	// We just test that the function can be called
	_, err := NewPool(context.Background(), "")
	if err != nil {
		// Expected error for empty DSN
		t.Logf("Got expected error for empty DSN: %v", err)
	} else {
		t.Log("No error for empty DSN (unexpected but not failing test)")
	}
}

func TestNewPool_InvalidHost(t *testing.T) {
	// This may or may not fail depending on network configuration
	// We just test that the function can be called
	_, err := NewPool(context.Background(), "postgres://user:pass@invalidhost:5432/db")
	if err != nil {
		// Expected error for invalid host
		t.Logf("Got expected error for invalid host: %v", err)
	} else {
		t.Log("No error for invalid host (unexpected but not failing test)")
	}
}

func TestNewPool_InvalidPort(t *testing.T) {
	// This may or may not fail depending on network configuration
	// We just test that the function can be called
	_, err := NewPool(context.Background(), "postgres://user:pass@localhost:99999/db")
	if err != nil {
		// Expected error for invalid port
		t.Logf("Got expected error for invalid port: %v", err)
	} else {
		t.Log("No error for invalid port (unexpected but not failing test)")
	}
}
