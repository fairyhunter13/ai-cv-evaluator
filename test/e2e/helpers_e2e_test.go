//go:build e2e

package e2e_test

import "os"

// getenv returns the value of the environment variable k or def if empty.
func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }
