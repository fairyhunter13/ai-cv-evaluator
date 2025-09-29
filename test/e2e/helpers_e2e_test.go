//go:build e2e

package e2e_test

import (
    "net/http"
    "os"
)

// getenv returns the value of the environment variable k or def if empty.
func getenv(k, def string) string { if v := os.Getenv(k); v != "" { return v }; return def }

// maybeBasicAuth sets HTTP Basic Auth on the request if ADMIN_USERNAME and ADMIN_PASSWORD are present.
func maybeBasicAuth(req *http.Request) {
    u := os.Getenv("ADMIN_USERNAME")
    p := os.Getenv("ADMIN_PASSWORD")
    // Fallback to defaults typically used in dev if not provided
    if u == "" { u = "admin" }
    if p == "" { p = "admin123" }
    req.SetBasicAuth(u, p)
}
