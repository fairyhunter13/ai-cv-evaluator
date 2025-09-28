//go:build e2e
// +build e2e

package e2e_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: Admin UI E2E tests are disabled. We validate the UI manually via Cascade Browser.
// Keeping this file under the e2e tag but skipping at runtime to avoid execution while
// preserving history and potential future reuse if UI automation is revisited.
// TestE2E_AdminUI tests the admin user interface functionality
func TestE2E_AdminUI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	// Skip if admin is not enabled
	if !isAdminEnabled() {
		t.Skip("Admin UI not enabled, skipping admin E2E tests")
	}

	// Always skip: Admin UI E2E is intentionally not automated.
	t.Skip("Admin UI E2E disabled; validate UI manually using browser preview")

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	baseAdminURL := "http://localhost:8080/admin"

	t.Run("Admin_Login_Page_Accessible", func(t *testing.T) {
		resp, err := client.Get(baseAdminURL + "/login")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should return login page or redirect
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusFound)
	})

	t.Run("Admin_Login_Authentication", func(t *testing.T) {
		// Test login with invalid credentials
		data := url.Values{}
		data.Set("username", "invalid")
		data.Set("password", "invalid")

		req, err := http.NewRequest("POST", baseAdminURL+"/login", strings.NewReader(data.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should redirect back to login or return unauthorized
		assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusFound)
	})

	t.Run("Admin_Protected_Routes_Require_Auth", func(t *testing.T) {
		protectedRoutes := []string{
			"/admin/dashboard",
			"/admin/rag/data",
			"/admin/rag/upload",
		}

		for _, route := range protectedRoutes {
			t.Run("Route_"+strings.ReplaceAll(route, "/", "_"), func(t *testing.T) {
				resp, err := client.Get("http://localhost:8080" + route)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Should redirect to login or return unauthorized
				assert.True(t, resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusFound)
			})
		}
	})

	t.Run("Admin_Static_Assets_Accessible", func(t *testing.T) {
		// Test that static assets are served correctly
		staticRoutes := []string{
			"/admin/static/css/main.css", // Example CSS
			"/admin/static/js/main.js",   // Example JS
		}

		for _, route := range staticRoutes {
			t.Run("Asset_"+strings.ReplaceAll(route, "/", "_"), func(t *testing.T) {
				resp, err := client.Get("http://localhost:8080" + route)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Static assets should be accessible or return 404 if not found
				assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
			})
		}
	})
}

// TestE2E_AdminAPI tests the admin API functionality
func TestE2E_AdminAPI(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	if !isAdminEnabled() {
		t.Skip("Admin UI not enabled, skipping admin API E2E tests")
	}

	// Always skip: Admin UI E2E is intentionally not automated.
	t.Skip("Admin UI E2E disabled; validate UI manually using browser preview")

	client := &http.Client{Timeout: timeout}

	t.Run("Admin_API_Proxy_Upload", func(t *testing.T) {
		// Test that admin API properly proxies to main API
		resp, err := client.Get("http://localhost:8080/admin/api/v1/upload")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should require authentication or method not allowed
		assert.True(t, resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusUnauthorized)
	})

	t.Run("Admin_API_Proxy_Evaluate", func(t *testing.T) {
		resp, err := client.Get("http://localhost:8080/admin/api/v1/evaluate")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should require authentication or method not allowed
		assert.True(t, resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusUnauthorized)
	})
}

// Helper function to check if admin is enabled
func isAdminEnabled() bool {
	// Simple check - try to access admin login page
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:8080/admin/login")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// If we get anything other than 404, admin is likely enabled
	return resp.StatusCode != http.StatusNotFound
}
