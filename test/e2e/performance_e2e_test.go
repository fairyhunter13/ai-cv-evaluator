//go:build e2e
// +build e2e

package e2e_test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_Performance tests performance characteristics and concurrent load
func TestE2E_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("Concurrent_Uploads", func(t *testing.T) {
		const numWorkers = 10
		const requestsPerWorker = 5

		var wg sync.WaitGroup
		results := make(chan result, numWorkers*requestsPerWorker)

		// Create test data
		cvContent := generateTestCV()
		projectContent := generateTestProject()

		startTime := time.Now()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < requestsPerWorker; j++ {
					uploadStart := time.Now()
					
					// Create upload request
					var buf bytes.Buffer
					writer := multipart.NewWriter(&buf)

					cvWriter, err := writer.CreateFormFile("cv", fmt.Sprintf("cv_%d_%d.txt", workerID, j))
					if err != nil {
						results <- result{err: err}
						continue
					}
					cvWriter.Write([]byte(cvContent))

					projWriter, err := writer.CreateFormFile("project", fmt.Sprintf("project_%d_%d.txt", workerID, j))
					if err != nil {
						results <- result{err: err}
						continue
					}
					projWriter.Write([]byte(projectContent))
					writer.Close()

					// Send request
					req, err := http.NewRequest("POST", "http://localhost:8080/v1/upload", &buf)
					if err != nil {
						results <- result{err: err}
						continue
					}
					req.Header.Set("Content-Type", writer.FormDataContentType())

					resp, err := client.Do(req)
					duration := time.Since(uploadStart)

					if err != nil {
						results <- result{err: err}
						continue
					}

					resp.Body.Close()
					results <- result{
						statusCode: resp.StatusCode,
						duration:   duration,
					}
				}
			}(i)
		}

		wg.Wait()
		close(results)

		totalDuration := time.Since(startTime)

		// Analyze results
		var successful, failed int
		var totalResponseTime time.Duration
		var maxResponseTime time.Duration

		for result := range results {
			if result.err != nil {
				failed++
				t.Logf("Request failed: %v", result.err)
				continue
			}

			if result.statusCode == http.StatusOK || result.statusCode == http.StatusAccepted {
				successful++
				totalResponseTime += result.duration
				if result.duration > maxResponseTime {
					maxResponseTime = result.duration
				}
			} else {
				failed++
			}
		}

		totalRequests := numWorkers * requestsPerWorker
		successRate := float64(successful) / float64(totalRequests) * 100
		avgResponseTime := totalResponseTime / time.Duration(successful)
		throughput := float64(totalRequests) / totalDuration.Seconds()

		t.Logf("Performance Results:")
		t.Logf("  Total Requests: %d", totalRequests)
		t.Logf("  Successful: %d", successful)
		t.Logf("  Failed: %d", failed)
		t.Logf("  Success Rate: %.2f%%", successRate)
		t.Logf("  Avg Response Time: %v", avgResponseTime)
		t.Logf("  Max Response Time: %v", maxResponseTime)
		t.Logf("  Throughput: %.2f req/sec", throughput)
		t.Logf("  Total Duration: %v", totalDuration)

		// Performance assertions
		assert.GreaterOrEqual(t, successRate, 80.0, "Success rate should be at least 80%")
		assert.LessOrEqual(t, avgResponseTime, 5*time.Second, "Average response time should be under 5 seconds")
		assert.LessOrEqual(t, maxResponseTime, 10*time.Second, "Max response time should be under 10 seconds")
	})

	t.Run("Memory_Stress_Large_Files", func(t *testing.T) {
		// Test with multiple large files to check memory handling
		const fileSize = 1024 * 1024 // 1MB
		largeContent := generateLargeContent(fileSize)

		for i := 0; i < 3; i++ {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			cvWriter, err := writer.CreateFormFile("cv", fmt.Sprintf("large_cv_%d.txt", i))
			require.NoError(t, err)
			cvWriter.Write([]byte(largeContent))

			projWriter, err := writer.CreateFormFile("project", fmt.Sprintf("large_project_%d.txt", i))
			require.NoError(t, err)
			projWriter.Write([]byte(largeContent))

			writer.Close()

			req, err := http.NewRequest("POST", "http://localhost:8080/v1/upload", &buf)
			require.NoError(t, err)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			startTime := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(startTime)

			require.NoError(t, err)
			resp.Body.Close()

			t.Logf("Large file %d: Status %d, Duration %v", i, resp.StatusCode, duration)

			// Should handle large files gracefully
			assert.True(t, resp.StatusCode == http.StatusOK || 
				resp.StatusCode == http.StatusAccepted ||
				resp.StatusCode == http.StatusRequestEntityTooLarge,
				"Large files should be handled gracefully")
		}
	})

	t.Run("Health_Endpoint_Performance", func(t *testing.T) {
		// Health endpoints should be fast
		endpoints := []string{"/healthz", "/readyz"}

		for _, endpoint := range endpoints {
			var totalDuration time.Duration
			const numChecks = 20

			for i := 0; i < numChecks; i++ {
				start := time.Now()
				resp, err := client.Get("http://localhost:8080" + endpoint)
				duration := time.Since(start)

				require.NoError(t, err)
				resp.Body.Close()

				totalDuration += duration
			}

			avgDuration := totalDuration / numChecks
			t.Logf("%s average response time: %v", endpoint, avgDuration)

			// Health checks should be very fast
			assert.LessOrEqual(t, avgDuration, 100*time.Millisecond,
				"Health endpoints should respond within 100ms")
		}
	})
}

// TestE2E_Reliability tests system reliability under various conditions
func TestE2E_Reliability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E tests in short mode")
	}

	client := &http.Client{Timeout: timeout}

	t.Run("Service_Availability", func(t *testing.T) {
		// Check that all critical endpoints are available
		endpoints := []struct {
			path   string
			method string
			expectOK bool
		}{
			{"/healthz", "GET", true},
			{"/readyz", "GET", true},
			{"/metrics", "GET", true},
			{"/v1/upload", "GET", false}, // Should return method not allowed, but service available
			{"/openapi.yaml", "GET", true},
		}

		for _, ep := range endpoints {
			req, err := http.NewRequest(ep.method, "http://localhost:8080"+ep.path, nil)
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err, "Endpoint %s should be reachable", ep.path)
			resp.Body.Close()

			if ep.expectOK {
				assert.Equal(t, http.StatusOK, resp.StatusCode,
					"Endpoint %s should return 200 OK", ep.path)
			} else {
				assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
					"Endpoint %s should not return 500", ep.path)
			}
		}
	})

	t.Run("Graceful_Error_Handling", func(t *testing.T) {
		// Test various error conditions
		errorCases := []struct {
			name     string
			method   string
			path     string
			body     io.Reader
			headers  map[string]string
			expected []int // acceptable status codes
		}{
			{
				name:     "InvalidJSON",
				method:   "POST",
				path:     "/v1/evaluate",
				body:     bytes.NewReader([]byte("invalid json")),
				headers:  map[string]string{"Content-Type": "application/json"},
				expected: []int{http.StatusBadRequest, http.StatusUnsupportedMediaType},
			},
			{
				name:     "MissingContentType",
				method:   "POST",
				path:     "/v1/upload",
				body:     bytes.NewReader([]byte("test")),
				expected: []int{http.StatusBadRequest, http.StatusUnsupportedMediaType},
			},
			{
				name:     "EmptyRequest",
				method:   "POST",
				path:     "/v1/upload",
				body:     bytes.NewReader([]byte("")),
				expected: []int{http.StatusBadRequest},
			},
		}

		for _, tc := range errorCases {
			t.Run(tc.name, func(t *testing.T) {
				req, err := http.NewRequest(tc.method, "http://localhost:8080"+tc.path, tc.body)
				require.NoError(t, err)

				for k, v := range tc.headers {
					req.Header.Set(k, v)
				}

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Should handle errors gracefully
				assert.Contains(t, tc.expected, resp.StatusCode,
					"Error case %s should return expected status code", tc.name)
			})
		}
	})
}

// Helper types and functions
type result struct {
	statusCode int
	duration   time.Duration
	err        error
}

func generateTestCV() string {
	return `John Doe
Software Engineer

EXPERIENCE:
- Senior Developer at TechCorp (2020-2023)
- Full-stack development with Go, React, PostgreSQL
- Led team of 5 developers
- Implemented microservices architecture

EDUCATION:
- BS Computer Science, University (2016-2020)

SKILLS:
- Programming: Go, Python, JavaScript, TypeScript
- Databases: PostgreSQL, Redis, MongoDB
- Cloud: AWS, Docker, Kubernetes
- Frameworks: React, Vue.js, Echo, Gin`
}

func generateTestProject() string {
	return `E-Commerce Platform

PROJECT OVERVIEW:
Built a scalable e-commerce platform serving 10,000+ users daily.

TECHNICAL STACK:
- Backend: Go with Echo framework
- Database: PostgreSQL with Redis caching
- Frontend: React with TypeScript
- Infrastructure: Docker, AWS ECS, RDS

KEY ACHIEVEMENTS:
- 99.9% uptime over 12 months
- Sub-200ms API response times
- Processed $1M+ in transactions
- Implemented real-time inventory management
- Built automated testing pipeline

ARCHITECTURE:
- Microservices with API Gateway
- Event-driven architecture using SQS
- Horizontal scaling with load balancers
- Database read replicas for performance`
}

func generateLargeContent(size int) string {
	const chunk = "This is a test content chunk that will be repeated to create large files. "
	chunkSize := len(chunk)
	repeats := size / chunkSize
	
	result := ""
	for i := 0; i < repeats; i++ {
		result += chunk
	}
	
	// Fill remaining bytes
	remaining := size - len(result)
	if remaining > 0 && remaining < chunkSize {
		result += chunk[:remaining]
	}
	
	return result
}
