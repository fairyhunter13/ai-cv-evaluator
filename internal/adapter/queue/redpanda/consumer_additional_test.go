package redpanda

import (
	"context"
	"testing"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

// minimalConsumer constructs a Consumer with just enough fields populated to test helper methods
func minimalConsumer() *Consumer {
	return &Consumer{
		groupID:    "group",
		topic:      "topic",
		minWorkers: 1,
		maxWorkers: 4,
		jobQueue:   make(chan *kgo.Record, 8),
		shutdown:   make(chan struct{}),
	}
}

func TestConsumer_WorkerHelpers_IncrementDecrementActiveWorkers(t *testing.T) {
	c := minimalConsumer()

	// initial should be zero
	require.Equal(t, 0, c.getActiveWorkers())

	c.incrementActiveWorkers()
	require.Equal(t, 1, c.getActiveWorkers())

	c.incrementActiveWorkers()
	require.Equal(t, 2, c.getActiveWorkers())

	c.decrementActiveWorkers()
	require.Equal(t, 1, c.getActiveWorkers())

	// decrement below zero is clamped
	c.decrementActiveWorkers()
	c.decrementActiveWorkers()
	require.Equal(t, 0, c.getActiveWorkers())
}

func TestConsumer_MinIntHelper(t *testing.T) {
	require.Equal(t, 1, minInt(1, 5))
	require.Equal(t, 2, minInt(10, 2))
}

func TestConsumer_GetHealthStatus_WithNilObservableClient(t *testing.T) {
	c := minimalConsumer()
	c.observableClient = nil

	status := c.GetHealthStatus()
	require.Equal(t, "unhealthy", status["status"])
	require.Contains(t, status["reason"].(string), "not initialized")
}

func TestConsumer_GetHealthStatus_WithObservableClient(t *testing.T) {
	c := minimalConsumer()
	// Use real integrated client with deterministic AdaptiveTimeout stats
	c.observableClient = observability.NewIntegratedObservableClient(
		observability.ConnectionTypeQueue,
		observability.OperationTypePoll,
		"broker:9092",
		"worker",
		100*time.Millisecond,
		100*time.Millisecond,
		100*time.Millisecond,
	)

	status := c.GetHealthStatus()
	require.Equal(t, "redpanda", status["consumer_type"])
	require.Equal(t, "group", status["group_id"])
	require.Equal(t, "topic", status["topic"])
}

func TestConsumer_IsHealthy_DelegatesToObservableClient(t *testing.T) {
	c := minimalConsumer()
	c.observableClient = observability.NewIntegratedObservableClient(
		observability.ConnectionTypeQueue,
		observability.OperationTypePoll,
		"broker:9092",
		"worker",
		100*time.Millisecond,
		100*time.Millisecond,
		100*time.Millisecond,
	)

	// By default success_rate is 0 so IsHealthy should be false
	require.False(t, c.IsHealthy())
}

func TestConsumer_WithRetryManager_SetsField(t *testing.T) {
	c := minimalConsumer()
	// nil RetryManager is fine for this test; we only assert wiring
	c.WithRetryManager(&RetryManager{})
	require.NotNil(t, c.retryManager)
}

func TestConsumer_ScaleWorkers_WithEmptyQueue_DoesNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := minimalConsumer()
	c.scaleWorkers(ctx)

	// With empty queue and initial activeWorkers==0, we should still respect minWorkers
	// but scaleWorkers does not spawn workers directly when queue is empty.
	require.Equal(t, 0, c.getActiveWorkers())
}
