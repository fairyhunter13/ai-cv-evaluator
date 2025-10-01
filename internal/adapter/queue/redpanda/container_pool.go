package redpanda

import (
	"context"
	"fmt"
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

// ContainerPool manages a pool of Redpanda containers
type ContainerPool struct {
	containers chan ContainerInfo
	poolSize   int
	created    bool
	once       sync.Once
	mu         sync.RWMutex
}

var (
	globalPool *ContainerPool
	poolOnce   sync.Once
)

// GetContainerPool returns the global container pool
func GetContainerPool() *ContainerPool {
	poolOnce.Do(func() {
		// Pool size should match or exceed parallel test count
		// Default to 6 containers (4 parallel + 2 buffer)
		globalPool = &ContainerPool{
			containers: make(chan ContainerInfo, 6),
			poolSize:   6,
		}
	})
	return globalPool
}

// InitializePool creates the container pool
func (p *ContainerPool) InitializePool(t *testing.T) error {
	var initErr error

	p.once.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.created {
			return
		}

		// Create containers concurrently
		var wg sync.WaitGroup
		errors := make([]error, p.poolSize)

		for i := 0; i < p.poolSize; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				container, broker, err := p.createContainer(t, id)
				if err != nil {
					errors[id] = err
					return
				}

				// Send container to pool
				select {
				case p.containers <- ContainerInfo{
					Container: container,
					Broker:    broker,
					ID:        id,
				}:
				default:
					// Pool is full, terminate container
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					_ = container.Terminate(ctx)
					errors[id] = fmt.Errorf("pool full")
				}
			}(i)
		}

		wg.Wait()

		// Check for any errors
		for _, err := range errors {
			if err != nil {
				initErr = err
				break
			}
		}

		if initErr == nil {
			p.created = true
		}
	})

	return initErr
}

// GetContainer acquires a container from the pool
func (p *ContainerPool) GetContainer(t *testing.T) (ContainerInfo, error) {
	p.mu.RLock()
	if !p.created {
		p.mu.RUnlock()
		if err := p.InitializePool(t); err != nil {
			return ContainerInfo{}, err
		}
		p.mu.RLock()
	}
	p.mu.RUnlock()

	select {
	case container := <-p.containers:
		return container, nil
	case <-time.After(30 * time.Second):
		return ContainerInfo{}, fmt.Errorf("timeout waiting for container from pool")
	}
}

// ReturnContainer returns a container to the pool
func (p *ContainerPool) ReturnContainer(container ContainerInfo) {
	select {
	case p.containers <- container:
		// Container returned successfully
	default:
		// Pool is full, terminate the container
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = container.Container.Terminate(ctx)
	}
}

// createContainer creates a single Redpanda container
func (p *ContainerPool) createContainer(_ *testing.T, id int) (tc.Container, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Calculate port for this container (19092 + id)
	port := 19092 + id

	req := tc.ContainerRequest{
		Image:        "redpandadata/redpanda:v24.3.7",
		ExposedPorts: []string{"9092/tcp", "9644/tcp"},
		Cmd: []string{
			"redpanda", "start",
			"--overprovisioned",
			"--smp", "1",
			"--memory", "256M", // Reduced memory per container
			"--reserve-memory", "0M",
			"--node-id", fmt.Sprintf("%d", id),
			"--check=false",
			"--kafka-addr", "PLAINTEXT://0.0.0.0:9092",
			"--advertise-kafka-addr", fmt.Sprintf("PLAINTEXT://127.0.0.1:%d", port),
			"--default-log-level=error",
			"--mode", "dev-container",
		},
		WaitingFor: wait.ForListeningPort("9092/tcp").WithStartupTimeout(30 * time.Second),
	}

	// Bind host port
	req.HostConfigModifier = func(hc *containerTypes.HostConfig) {
		if hc.PortBindings == nil {
			hc.PortBindings = nat.PortMap{}
		}
		hc.PortBindings[nat.Port("9092/tcp")] = []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%d", port)},
		}
	}

	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to start redpanda container %d: %v", id, err)
	}

	broker := fmt.Sprintf("localhost:%d", port)
	return container, broker, nil
}

// CleanupPool terminates all containers in the pool
func (p *ContainerPool) CleanupPool() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.created {
		return
	}

	// Close the channel to signal no more containers will be added
	close(p.containers)

	// Terminate all remaining containers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for container := range p.containers {
		if err := container.Container.Terminate(ctx); err != nil {
			fmt.Printf("Warning: failed to terminate container %d: %v\n", container.ID, err)
		}
	}

	p.created = false
}

// GetPoolStats returns pool statistics
func (p *ContainerPool) GetPoolStats() (available, total int) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.created {
		return 0, p.poolSize
	}

	available = len(p.containers)
	return available, p.poolSize
}
