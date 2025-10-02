// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
package redpanda

import (
	"context"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ContainerInfo holds container and broker information
type ContainerInfo struct {
	Container tc.Container
	Broker    string
	ID        int
}

// ContainerPool manages a single Redpanda container shared across all tests
type ContainerPool struct {
	container ContainerInfo
	created   chan struct{} // Channel to signal container is ready
	once      sync.Once
	mu        sync.RWMutex // Protect container access
	refCount  int          // Reference counter to track active users
	cleanupMu sync.Mutex   // Mutex to prevent concurrent cleanup
}

var (
	globalPool *ContainerPool
	poolOnce   sync.Once
)

// detectSystemResources determines optimal container configuration based on system resources
func detectSystemResources() (poolSize int, memoryLimit int64) {
	// Single container approach - much more efficient
	poolSize = 1 // Single container for all tests

	// Get system information
	numCPU := runtime.NumCPU()

	// Allocate more memory to single container based on system resources
	if numCPU >= 8 {
		memoryLimit = 1024 * 1024 * 1024 // 1GB for high-end systems
	} else if numCPU >= 4 {
		memoryLimit = 768 * 1024 * 1024 // 768MB for mid-range systems
	} else {
		memoryLimit = 512 * 1024 * 1024 // 512MB for low-end systems
	}

	// Check for Docker resource constraints
	if os.Getenv("DOCKER_RESOURCE_LIMITS") == "true" {
		memoryLimit = 512 * 1024 * 1024 // 512MB minimum
	}

	return poolSize, memoryLimit
}

// GetContainerPool returns the global container pool
func GetContainerPool() *ContainerPool {
	poolOnce.Do(func() {
		fmt.Printf("🏗️ Creating global container pool (first time only)\n")
		globalPool = &ContainerPool{
			created: make(chan struct{}),
		}

		// Note: Removed automatic cleanup goroutine to prevent race conditions
		// with parallel tests. Cleanup is now handled by TestMain only.
	})
	return globalPool
}

// InitializePool creates the single shared container
func (p *ContainerPool) InitializePool(t *testing.T) error {
	var initErr error

	p.once.Do(func() {
		t.Logf("🔧 Initializing shared container pool (first time only)")
		// Create single container with retry logic
		var container tc.Container
		var broker string
		var err error

		// Optimized retry logic with faster backoff for test environments
		maxRetries := 3              // Reduced from 5 for faster failure
		baseDelay := 1 * time.Second // Reduced from 2s

		for attempt := 0; attempt < maxRetries; attempt++ {
			container, broker, err = p.createContainer(t, 0)
			if err == nil {
				break
			}

			// Log the attempt for debugging
			t.Logf("Container attempt %d failed: %v", attempt+1, err)

			if attempt < maxRetries-1 {
				// Linear backoff for faster retry in test environments
				delay := baseDelay * time.Duration(attempt+1) // 1s, 2s, 3s
				time.Sleep(delay)
			}
		}

		if err != nil {
			initErr = fmt.Errorf("failed to create shared container after %d attempts: %v", maxRetries, err)
			return
		}

		// Store the single container
		p.container = ContainerInfo{
			Container: container,
			Broker:    broker,
			ID:        0,
		}

		// Signal that the container is ready
		close(p.created)
		t.Logf("✅ Shared Redpanda container initialized successfully at %s", broker)
	})

	return initErr
}

// GetContainer returns the shared container
func (p *ContainerPool) GetContainer(_ *testing.T) (ContainerInfo, error) {
	// Wait for container to be ready
	select {
	case <-p.created:
		// Container is ready, increment reference counter
		p.mu.Lock()
		p.refCount++
		container := p.container
		p.mu.Unlock()
		return container, nil
	case <-time.After(2 * time.Minute):
		return ContainerInfo{}, fmt.Errorf("timeout waiting for container initialization")
	}
}

// ReturnContainer decrements the reference counter
func (p *ContainerPool) ReturnContainer(_ ContainerInfo) {
	// Decrement reference counter when test is done with container
	p.mu.Lock()
	if p.refCount > 0 {
		p.refCount--
	}
	p.mu.Unlock()
}

// createContainer creates a single Redpanda container
func (p *ContainerPool) createContainer(_ *testing.T, id int) (tc.Container, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Retry container creation up to 3 times for transient failures
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		container, broker, err := p.createContainerAttempt(ctx, id)
		if err == nil {
			return container, broker, nil
		}
		lastErr = err

		// Wait before retry (exponential backoff)
		if attempt < maxRetries-1 {
			waitTime := time.Duration(attempt+1) * 2 * time.Second
			time.Sleep(waitTime)
		}
	}

	return nil, "", fmt.Errorf("failed to create container after %d attempts: %w", maxRetries, lastErr)
}

// createContainerAttempt makes a single attempt to create a container
func (p *ContainerPool) createContainerAttempt(ctx context.Context, id int) (tc.Container, string, error) {
	// Use dynamic port allocation to avoid conflicts
	// Dynamically find an available port on the host for 9092/tcp
	getAvailablePort := func() (int, error) {
		// Try multiple times to find an available port
		maxRetries := 5
		for attempt := 0; attempt < maxRetries; attempt++ {
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				if attempt == maxRetries-1 {
					return 0, fmt.Errorf("failed to find available port after %d attempts: %w", maxRetries, err)
				}
				// Brief delay before retry
				time.Sleep(100 * time.Millisecond)
				continue
			}
			defer func() {
				_ = l.Close() // Ignore error in cleanup
			}()
			addr := l.Addr().(*net.TCPAddr)
			return addr.Port, nil
		}
		return 0, fmt.Errorf("failed to find available port after %d attempts", maxRetries)
	}

	// Get the available port before creating the request
	availablePort, err := getAvailablePort()
	if err != nil {
		return nil, "", fmt.Errorf("failed to find available port for Redpanda container %d: %w", id, err)
	}
	port := availablePort

	req := tc.ContainerRequest{
		Image:        "docker.redpanda.com/redpandadata/redpanda:v24.3.1",
		ExposedPorts: []string{"9092/tcp", "9644/tcp"},
		Cmd: []string{
			"redpanda", "start",
			"--kafka-addr", "PLAINTEXT://0.0.0.0:9092",
			"--advertise-kafka-addr", fmt.Sprintf("PLAINTEXT://127.0.0.1:%d", port),
			"--mode", "dev-container",
			"--smp", "1",
			"--default-log-level=error",
			"--overprovisioned",
			"--unsafe-bypass-fsync=true",
			"--lock-memory=false",
			"--reserve-memory=0M",
		},
		Env: map[string]string{
			"REDPANDA_DEVELOPER_MODE": "true",
			"REDPANDA_LOG_LEVEL":      "error",
			"REDPANDA_AIO_MAX_NR":     "0",    // Disable AIO to avoid macOS issues
			"REDPANDA_FAST_STARTUP":   "true", // Custom optimization flag
		},
		WaitingFor: wait.ForListeningPort("9092/tcp").WithStartupTimeout(15 * time.Second), // Reduced from 30s
	}

	// Bind host port and configure container for testing
	req.HostConfigModifier = func(hc *containerTypes.HostConfig) {
		if hc.PortBindings == nil {
			hc.PortBindings = nat.PortMap{}
		}
		hc.PortBindings[nat.Port("9092/tcp")] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", port)},
		}

		// Configure container for testing environment with optimized resource limits
		_, memoryLimit := detectSystemResources()
		hc.Resources = containerTypes.Resources{
			Memory:     memoryLimit, // Use detected memory limit
			MemorySwap: -1,          // Disable swap
			CPUShares:  512,         // Increased CPU shares for faster processing
			NanoCPUs:   500000000,   // 0.5 CPU cores (500 million nanoseconds) - increased for faster startup
		}

		// Add additional Docker configuration for macOS compatibility
		hc.Sysctls = map[string]string{
			"net.core.somaxconn": "1024",
		}

		// Disable swap to avoid issues
		hc.MemorySwap = -1

		// Add ulimits to prevent resource issues
		hc.Ulimits = []*containerTypes.Ulimit{
			{Name: "nofile", Soft: 2048, Hard: 2048}, // Increased file descriptor limit
			{Name: "nproc", Soft: 2048, Hard: 2048},  // Increased process limit
			{Name: "memlock", Soft: -1, Hard: -1},    // Unlimited memory lock
		}
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return nil, "", fmt.Errorf("timeout starting redpanda container %d (2 minutes): %w", id, err)
		}
		return nil, "", fmt.Errorf("failed to start redpanda container %d: %w", id, err)
	}

	broker := fmt.Sprintf("localhost:%d", port)
	return container, broker, nil
}

// CleanupPool terminates the shared container
func (p *ContainerPool) CleanupPool() {
	// Prevent concurrent cleanup
	p.cleanupMu.Lock()
	defer p.cleanupMu.Unlock()

	// Check if container was ever created
	select {
	case <-p.created:
		// Container was created, proceed with cleanup
	default:
		// Container was never created, nothing to clean up
		return
	}

	// Wait a bit for any active tests to finish
	// This gives parallel tests time to complete
	time.Sleep(1 * time.Second)

	// Check reference count - only cleanup if no active users
	p.mu.RLock()
	refCount := p.refCount
	container := p.container
	p.mu.RUnlock()

	if refCount > 0 {
		fmt.Printf("Warning: %d active references to container, skipping cleanup\n", refCount)
		return
	}

	// Terminate the single shared container
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if container.Container != nil {
		if err := container.Container.Terminate(ctx); err != nil {
			fmt.Printf("Warning: failed to terminate shared container during cleanup: %v\n", err)
		} else {
			fmt.Printf("Shared Redpanda container terminated successfully\n")
		}
	}
}

// GetPoolStats returns pool statistics
func (p *ContainerPool) GetPoolStats() (available, total int) {
	// Check if container was created
	select {
	case <-p.created:
		// Container is ready
		return 1, 1 // Single container available
	default:
		// Container not ready yet
		return 0, 1 // Single container total
	}
}

// GetRefCount returns the current reference count for debugging
func (p *ContainerPool) GetRefCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.refCount
}
