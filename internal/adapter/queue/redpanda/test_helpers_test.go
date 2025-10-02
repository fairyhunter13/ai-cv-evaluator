// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
package redpanda

import (
	"testing"
)

// getContainerBroker gets the broker address from the shared container
func getContainerBroker(t *testing.T) string {
	pool := GetContainerPool()
	if err := pool.InitializePool(t); err != nil {
		t.Fatalf("Failed to initialize container pool: %v", err)
	}

	containerInfo, err := pool.GetContainer(t)
	if err != nil {
		t.Fatalf("Failed to get container: %v", err)
	}

	return containerInfo.Broker
}
