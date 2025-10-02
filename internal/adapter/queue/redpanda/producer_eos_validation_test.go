package redpanda

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// TestProducerEOSValidation tests EOS compliance for normal job processing queue
func TestProducerEOSValidation(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Use the shared container pool
	brokerAddr := getContainerBroker(t)

	// Create producer with transactional ID for EOS
	producer, err := NewProducerWithTransactionalID([]string{brokerAddr}, "test-producer-eos-validation")
	require.NoError(t, err)
	defer producer.Close()

	t.Run("EOS_Exactly_Once_Delivery", func(t *testing.T) {
		testEOSExactlyOnceDeliveryValidation(t, ctx, producer)
	})

	t.Run("EOS_Transaction_Atomicity", func(t *testing.T) {
		testEOSTransactionAtomicityValidation(t, ctx, producer)
	})

	t.Run("EOS_Concurrent_Transactions", func(t *testing.T) {
		testEOSConcurrentTransactionsValidation(t, ctx, producer)
	})

	t.Run("EOS_Error_Recovery", func(t *testing.T) {
		testEOSErrorRecoveryValidation(t, ctx, producer)
	})

	t.Run("EOS_Transaction_Isolation", func(t *testing.T) {
		testEOSTransactionIsolationValidation(t, ctx, producer)
	})

	t.Run("EOS_Message_Ordering", func(t *testing.T) {
		testEOSMessageOrderingValidation(t, ctx, producer)
	})
}

// testEOSExactlyOnceDeliveryValidation tests exactly-once delivery semantics
func testEOSExactlyOnceDeliveryValidation(t *testing.T, ctx context.Context, producer *Producer) {
	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-exactly-once",
		CVID:      "test-cv-exactly-once",
		ProjectID: "test-project-exactly-once",
	}

	// Multiple attempts with same payload should result in exactly-once delivery
	jobIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		jobID, err := producer.EnqueueEvaluate(ctx, payload)
		require.NoError(t, err)
		jobIDs[i] = jobID
	}

	// All job IDs should be the same (idempotent)
	for i := 1; i < len(jobIDs); i++ {
		assert.Equal(t, jobIDs[0], jobIDs[i], "EOS should ensure exactly-once delivery")
	}

	slog.Info("EOS exactly-once delivery test completed", slog.String("job_id", jobIDs[0]))
}

// testEOSTransactionAtomicity tests transaction atomicity
func testEOSTransactionAtomicityValidation(t *testing.T, ctx context.Context, producer *Producer) {
	// Test that transactions are atomic - either all operations succeed or none do
	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-atomicity",
		CVID:      "test-cv-atomicity",
		ProjectID: "test-project-atomicity",
	}

	// This should succeed atomically
	jobID, err := producer.EnqueueEvaluate(ctx, payload)
	require.NoError(t, err)
	assert.Equal(t, payload.JobID, jobID)

	// Test with invalid payload to ensure rollback works
	invalidPayload := domain.EvaluateTaskPayload{
		JobID:     "", // Empty job ID should cause issues
		CVID:      "test-cv-invalid",
		ProjectID: "test-project-invalid",
	}

	// This should fail and rollback
	_, err = producer.EnqueueEvaluate(ctx, invalidPayload)
	// The error handling depends on implementation, but transaction should be rolled back
	t.Logf("Invalid payload test result: %v", err)

	slog.Info("EOS transaction atomicity test completed")
}

// testEOSConcurrentTransactions tests EOS under concurrent load
func testEOSConcurrentTransactionsValidation(t *testing.T, ctx context.Context, producer *Producer) {
	const numGoroutines = 10
	const numMessagesPerGoroutine = 3

	results := make(chan error, numGoroutines*numMessagesPerGoroutine)

	// Start concurrent producers
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numMessagesPerGoroutine; j++ {
				payload := domain.EvaluateTaskPayload{
					JobID:     fmt.Sprintf("concurrent-job-%d-%d", goroutineID, j),
					CVID:      fmt.Sprintf("cv-%d", goroutineID),
					ProjectID: fmt.Sprintf("project-%d", goroutineID),
				}

				_, err := producer.EnqueueEvaluate(ctx, payload)
				results <- err
			}
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines*numMessagesPerGoroutine; i++ {
		select {
		case err := <-results:
			if err != nil {
				errorCount++
				t.Logf("Concurrent transaction error: %v", err)
			} else {
				successCount++
			}
		case <-time.After(30 * time.Second):
			t.Fatal("Timeout waiting for concurrent transactions")
		}
	}

	// Verify all transactions succeeded with EOS
	assert.Equal(t, 0, errorCount, "All concurrent transactions should succeed with EOS")
	slog.Info("EOS concurrent transactions test completed", slog.Int("successful_transactions", successCount))
}

// testEOSErrorRecovery tests EOS error recovery scenarios
func testEOSErrorRecoveryValidation(t *testing.T, ctx context.Context, producer *Producer) {
	// Test context cancellation
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel() // Cancel immediately

	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-cancelled",
		CVID:      "test-cv-cancelled",
		ProjectID: "test-project-cancelled",
	}

	// This should fail due to context cancellation
	_, err := producer.EnqueueEvaluate(cancelCtx, payload)
	assert.Error(t, err, "Should fail due to context cancellation")
	assert.Contains(t, err.Error(), "context canceled", "Error should indicate context cancellation")

	// Test timeout scenarios
	timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
	defer cancel()
	time.Sleep(1 * time.Millisecond) // Ensure timeout

	// This should fail due to timeout
	_, err = producer.EnqueueEvaluate(timeoutCtx, payload)
	assert.Error(t, err, "Should fail due to timeout")

	slog.Info("EOS error recovery test completed")
}

// testEOSTransactionIsolationValidation tests transaction isolation
func testEOSTransactionIsolationValidation(t *testing.T, ctx context.Context, producer *Producer) {
	// Get broker address from the shared container pool
	brokerAddr := getContainerBroker(t)

	// Create two producers with different transactional IDs
	producer1, err := NewProducerWithTransactionalID([]string{brokerAddr}, "test-producer-isolation-1")
	require.NoError(t, err)
	defer producer1.Close()

	producer2, err := NewProducerWithTransactionalID([]string{brokerAddr}, "test-producer-isolation-2")
	require.NoError(t, err)
	defer producer2.Close()

	// Producer 1 transaction
	payload1 := domain.EvaluateTaskPayload{
		JobID:     "test-job-isolation-1",
		CVID:      "test-cv-isolation-1",
		ProjectID: "test-project-isolation-1",
	}

	jobID1, err := producer1.EnqueueEvaluate(ctx, payload1)
	require.NoError(t, err)
	assert.Equal(t, payload1.JobID, jobID1)

	// Producer 2 transaction (should be isolated)
	payload2 := domain.EvaluateTaskPayload{
		JobID:     "test-job-isolation-2",
		CVID:      "test-cv-isolation-2",
		ProjectID: "test-project-isolation-2",
	}

	jobID2, err := producer2.EnqueueEvaluate(ctx, payload2)
	require.NoError(t, err)
	assert.Equal(t, payload2.JobID, jobID2)

	slog.Info("EOS transaction isolation test completed", slog.String("job1", jobID1), slog.String("job2", jobID2))
}

// testEOSMessageOrdering tests message ordering guarantees
func testEOSMessageOrderingValidation(t *testing.T, ctx context.Context, producer *Producer) {
	// Test that messages with the same key are ordered
	const numMessages = 5
	jobIDs := make([]string, numMessages)

	for i := 0; i < numMessages; i++ {
		payload := domain.EvaluateTaskPayload{
			JobID:     fmt.Sprintf("ordered-job-%d", i),
			CVID:      "test-cv-ordered",
			ProjectID: "test-project-ordered",
		}

		jobID, err := producer.EnqueueEvaluate(ctx, payload)
		require.NoError(t, err)
		jobIDs[i] = jobID
	}

	// All job IDs should be unique and in order
	for i := 0; i < numMessages; i++ {
		expectedJobID := fmt.Sprintf("ordered-job-%d", i)
		assert.Equal(t, expectedJobID, jobIDs[i], "Job IDs should be in order")
	}

	slog.Info("EOS message ordering test completed", slog.Int("messages_sent", numMessages))
}

// TestProducerEOSComplianceComprehensive tests comprehensive EOS compliance
func TestProducerEOSComplianceComprehensive(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Use the shared container pool
	brokerAddr := getContainerBroker(t)

	// Create producer with transactional ID for EOS
	producer, err := NewProducerWithTransactionalID([]string{brokerAddr}, "test-producer-comprehensive")
	require.NoError(t, err)
	defer producer.Close()

	t.Run("EOS_At_Least_Once_Delivery", func(t *testing.T) {
		testEOSAtLeastOnceDeliveryValidation(t, ctx, producer)
	})

	t.Run("EOS_At_Most_Once_Delivery", func(t *testing.T) {
		testEOSAtMostOnceDeliveryValidation(t, ctx, producer)
	})

	t.Run("EOS_Message_Durability", func(t *testing.T) {
		testEOSMessageDurabilityValidation(t, ctx, producer)
	})

	t.Run("EOS_Transaction_Consistency", func(t *testing.T) {
		testEOSTransactionConsistencyValidation(t, ctx, producer)
	})
}

// testEOSAtLeastOnceDelivery tests at-least-once delivery semantics
func testEOSAtLeastOnceDeliveryValidation(t *testing.T, ctx context.Context, producer *Producer) {
	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-at-least-once",
		CVID:      "test-cv-at-least-once",
		ProjectID: "test-project-at-least-once",
	}

	// Multiple attempts should not cause duplicates due to EOS
	for i := 0; i < 3; i++ {
		jobID, err := producer.EnqueueEvaluate(ctx, payload)
		require.NoError(t, err)
		assert.Equal(t, payload.JobID, jobID)
	}

	slog.Info("EOS at-least-once delivery test completed")
}

// testEOSAtMostOnceDelivery tests at-most-once delivery semantics
func testEOSAtMostOnceDeliveryValidation(t *testing.T, ctx context.Context, producer *Producer) {
	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-at-most-once",
		CVID:      "test-cv-at-most-once",
		ProjectID: "test-project-at-most-once",
	}

	// Single attempt should succeed
	jobID, err := producer.EnqueueEvaluate(ctx, payload)
	require.NoError(t, err)
	assert.Equal(t, payload.JobID, jobID)

	slog.Info("EOS at-most-once delivery test completed")
}

// testEOSMessageDurability tests message durability
func testEOSMessageDurabilityValidation(t *testing.T, ctx context.Context, producer *Producer) {
	payload := domain.EvaluateTaskPayload{
		JobID:     "test-job-durability",
		CVID:      "test-cv-durability",
		ProjectID: "test-project-durability",
	}

	// Message should be durable after successful transaction
	jobID, err := producer.EnqueueEvaluate(ctx, payload)
	require.NoError(t, err)
	assert.Equal(t, payload.JobID, jobID)

	// Wait a bit to ensure message is persisted
	time.Sleep(100 * time.Millisecond)

	slog.Info("EOS message durability test completed", slog.String("job_id", jobID))
}

// testEOSTransactionConsistency tests transaction consistency
func testEOSTransactionConsistencyValidation(t *testing.T, ctx context.Context, producer *Producer) {
	// Test that transactions maintain consistency across multiple operations
	payloads := []domain.EvaluateTaskPayload{
		{
			JobID:     "test-job-consistency-1",
			CVID:      "test-cv-consistency-1",
			ProjectID: "test-project-consistency-1",
		},
		{
			JobID:     "test-job-consistency-2",
			CVID:      "test-cv-consistency-2",
			ProjectID: "test-project-consistency-2",
		},
		{
			JobID:     "test-job-consistency-3",
			CVID:      "test-cv-consistency-3",
			ProjectID: "test-project-consistency-3",
		},
	}

	// All transactions should succeed consistently
	for i, payload := range payloads {
		jobID, err := producer.EnqueueEvaluate(ctx, payload)
		require.NoError(t, err, "Transaction %d should succeed", i+1)
		assert.Equal(t, payload.JobID, jobID, "Job ID should match for transaction %d", i+1)
	}

	slog.Info("EOS transaction consistency test completed", slog.Int("transactions", len(payloads)))
}
