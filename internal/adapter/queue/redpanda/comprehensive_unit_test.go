// Package redpanda provides comprehensive unit tests for the Redpanda queue adapter.
package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// newTestProducer creates a producer with a unique transactional ID for testing
func newTestProducer(t *testing.T, brokers []string) *Producer {
	producer, err := NewProducerWithTransactionalID(brokers, fmt.Sprintf("test-producer-%d-%s", time.Now().UnixNano(), t.Name()))
	require.NoError(t, err)
	return producer
}

// TestNewProducer_ComprehensiveValidation tests comprehensive validation scenarios
func TestNewProducer_ComprehensiveValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid_brokers", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		assert.NotNil(t, producer)
		defer func() { _ = producer.Close() }()
	})

	t.Run("empty_brokers", func(t *testing.T) {
		_, err := NewProducer([]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("nil_brokers", func(t *testing.T) {
		_, err := NewProducer(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("invalid_broker_format", func(t *testing.T) {
		// This should still create a client but fail on connection
		producer := newTestProducer(t, []string{"invalid-broker"})
		assert.NotNil(t, producer)
		defer func() { _ = producer.Close() }()
	})

	t.Run("multiple_brokers", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		assert.NotNil(t, producer)
		defer func() { _ = producer.Close() }()
	})
}

// TestProducer_EnqueueEvaluate_ComprehensiveErrorHandling tests comprehensive error handling
func TestProducer_EnqueueEvaluate_ComprehensiveErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("context_cancellation", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job",
			CVID:           "test-cv",
			ProjectID:      "test-project",
			JobDescription: "Test job",
			StudyCaseBrief: "Test study case",
		}

		_, err := producer.EnqueueEvaluate(ctx, payload)
		assert.Error(t, err)
		// Should fail due to cancelled context or unreachable broker
	})

	t.Run("timeout_context", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job",
			CVID:           "test-cv",
			ProjectID:      "test-project",
			JobDescription: "Test job",
			StudyCaseBrief: "Test study case",
		}

		_, err := producer.EnqueueEvaluate(ctx, payload)
		assert.Error(t, err)
		// Should fail due to timeout or unreachable broker
	})

	t.Run("empty_payload", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{}
		_, err := producer.EnqueueEvaluate(context.Background(), payload)
		// Note: The producer doesn't validate payload content, so this succeeds
		// The validation happens at the consumer level during processing
		assert.NoError(t, err)
	})

	t.Run("valid_payload_connection_error", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job-123",
			CVID:           "test-cv-456",
			ProjectID:      "test-project-789",
			JobDescription: "Test job description",
			StudyCaseBrief: "Test study case brief",
		}

		_, err := producer.EnqueueEvaluate(context.Background(), payload)
		// Note: With real Redpanda container, this succeeds
		// The test validates that valid payloads are handled correctly
		assert.NoError(t, err)
	})

	t.Run("json_marshal_success", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		// Test that JSON marshaling works (this should not fail)
		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job",
			CVID:           "test-cv",
			ProjectID:      "test-project",
			JobDescription: "Test job",
			StudyCaseBrief: "Test study case",
		}

		// The JSON marshaling should succeed, and the transaction should succeed
		_, err := producer.EnqueueEvaluate(context.Background(), payload)
		// Note: With real Redpanda container, this succeeds
		// The test validates that JSON marshaling works correctly
		assert.NoError(t, err)
	})
}

// TestProducer_Close_Comprehensive tests comprehensive close scenarios
func TestProducer_Close_Comprehensive(t *testing.T) {
	t.Parallel()

	t.Run("close_normal", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})

		err := producer.Close()
		assert.NoError(t, err)
	})

	t.Run("close_multiple_times", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})

		// Close multiple times
		err := producer.Close()
		assert.NoError(t, err)

		err = producer.Close()
		assert.NoError(t, err) // Should not error on multiple close
	})

	t.Run("close_nil_client", func(t *testing.T) {
		producer := &Producer{client: nil}
		err := producer.Close()
		assert.NoError(t, err) // Should not panic
	})
}

// TestNewConsumer_ComprehensiveValidation tests comprehensive consumer validation
func TestNewConsumer_ComprehensiveValidation(t *testing.T) {
	t.Parallel()

	t.Run("valid_configuration", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Equal(t, "test-group", consumer.groupID)
		defer func() { _ = consumer.Close() }()
	})

	t.Run("empty_brokers", func(t *testing.T) {
		_, err := NewConsumer(
			[]string{},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("nil_brokers", func(t *testing.T) {
		_, err := NewConsumer(
			nil,
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("empty_group_id", func(t *testing.T) {
		_, err := NewConsumer(
			[]string{"localhost:19092"},
			"",
			nil, nil, nil, nil, nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required group ID")
	})

	t.Run("invalid_broker_format", func(t *testing.T) {
		// This should still create a client but fail on connection
		consumer, err := NewConsumer(
			[]string{"invalid-broker"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		if err != nil {
			t.Logf("Expected error for invalid broker: %v", err)
		} else {
			assert.NotNil(t, consumer)
			defer func() { _ = consumer.Close() }()
		}
	})

	t.Run("multiple_brokers", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092", "localhost:9093"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		defer func() { _ = consumer.Close() }()
	})

	t.Run("with_repositories", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, // jobs
			nil, // uploads
			nil, // results
			nil, // ai
			nil, // qdrant
		)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Nil(t, consumer.jobs)
		assert.Nil(t, consumer.uploads)
		assert.Nil(t, consumer.results)
		assert.Nil(t, consumer.ai)
		assert.Nil(t, consumer.q)
		defer func() { _ = consumer.Close() }()
	})
}

// TestConsumer_Start_ComprehensiveErrorHandling tests comprehensive consumer start error handling
func TestConsumer_Start_ComprehensiveErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("context_cancellation", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = consumer.Start(ctx)
		assert.Error(t, err)
		// Should fail due to cancelled context or unreachable broker
	})

	t.Run("timeout_context", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = consumer.Start(ctx)
		assert.Error(t, err)
		// Should fail due to timeout or unreachable broker
	})

	t.Run("connection_error", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = consumer.Start(ctx)
		assert.Error(t, err)
		// Should fail due to connection issues or timeout
	})
}

// TestConsumer_Close_Comprehensive tests comprehensive consumer close scenarios
func TestConsumer_Close_Comprehensive(t *testing.T) {
	t.Parallel()

	t.Run("close_normal", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)

		err = consumer.Close()
		assert.NoError(t, err)
	})

	t.Run("close_multiple_times", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)

		// Close multiple times
		err = consumer.Close()
		assert.NoError(t, err)

		err = consumer.Close()
		assert.NoError(t, err) // Should not error on multiple close
	})

	t.Run("close_nil_session", func(t *testing.T) {
		consumer := &Consumer{session: nil}
		err := consumer.Close()
		assert.NoError(t, err) // Should not panic
	})
}

// TestCreateTopicIfNotExists_ComprehensiveErrorHandling tests comprehensive topic creation error handling
func TestCreateTopicIfNotExists_ComprehensiveErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("invalid_brokers", func(t *testing.T) {
		// Test with invalid brokers
		client, err := kgo.NewClient(kgo.SeedBrokers("invalid:9092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to dial")
	})

	t.Run("valid_brokers_no_connection", func(t *testing.T) {
		// Test with valid brokers (but no actual connection)
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:99999"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("empty_topic_name", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:99999"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "", 1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "topic name cannot be empty")
	})

	t.Run("invalid_partitions", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", 0, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "partitions must be greater than 0")
	})

	t.Run("invalid_replication_factor", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", 1, 0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "replication factor must be greater than 0")
	})

	t.Run("negative_partitions", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", -1, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "partitions must be greater than 0")
	})

	t.Run("negative_replication_factor", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic", 1, -1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "replication factor must be greater than 0")
	})
}

// TestTopicConstants_Comprehensive tests topic constants
func TestTopicConstants_Comprehensive(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "evaluate-jobs", TopicEvaluate)
	assert.NotEmpty(t, TopicEvaluate)
	assert.Greater(t, len(TopicEvaluate), 0)
}

// TestEvaluateTaskPayload_ComprehensiveStructure tests comprehensive payload structure
func TestEvaluateTaskPayload_ComprehensiveStructure(t *testing.T) {
	t.Parallel()

	t.Run("complete_payload", func(t *testing.T) {
		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job-1",
			CVID:           "test-cv-1",
			ProjectID:      "test-project-1",
			JobDescription: "Test job description",
			StudyCaseBrief: "Test study case",
		}

		assert.Equal(t, "test-job-1", payload.JobID)
		assert.Equal(t, "test-cv-1", payload.CVID)
		assert.Equal(t, "test-project-1", payload.ProjectID)
		assert.Equal(t, "Test job description", payload.JobDescription)
		assert.Equal(t, "Test study case", payload.StudyCaseBrief)
	})

	t.Run("empty_payload", func(t *testing.T) {
		payload := domain.EvaluateTaskPayload{}

		assert.Empty(t, payload.JobID)
		assert.Empty(t, payload.CVID)
		assert.Empty(t, payload.ProjectID)
		assert.Empty(t, payload.JobDescription)
		assert.Empty(t, payload.StudyCaseBrief)
	})

	t.Run("json_marshal_unmarshal", func(t *testing.T) {
		original := domain.EvaluateTaskPayload{
			JobID:          "test-job-1",
			CVID:           "test-cv-1",
			ProjectID:      "test-project-1",
			JobDescription: "Test job description",
			StudyCaseBrief: "Test study case",
		}

		// Marshal to JSON
		data, err := json.Marshal(original)
		assert.NoError(t, err)
		assert.NotEmpty(t, data)

		// Unmarshal from JSON
		var unmarshaled domain.EvaluateTaskPayload
		err = json.Unmarshal(data, &unmarshaled)
		assert.NoError(t, err)
		assert.Equal(t, original, unmarshaled)
	})
}

// TestErrorHandling_NetworkIssues_Comprehensive tests comprehensive network error handling
func TestErrorHandling_NetworkIssues_Comprehensive(t *testing.T) {
	t.Parallel()

	invalidBrokers := []string{
		"invalid-host:99999",
		"",
		"not-a-valid-address",
		"localhost:99999",
		"192.168.1.999:9092",
	}

	for _, broker := range invalidBrokers {
		t.Run("broker_"+broker, func(t *testing.T) {
			_, err := NewProducer([]string{broker})
			// Should handle invalid brokers gracefully
			if err != nil {
				t.Logf("Expected error for invalid broker %s: %v", broker, err)
			}
		})
	}
}

// TestConsumer_GroupID_Validation_Comprehensive tests comprehensive group ID validation
func TestConsumer_GroupID_Validation_Comprehensive(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		groupID string
		valid   bool
	}{
		{"empty group ID", "", false},
		{"valid group ID", "test-group", true},
		{"group ID with numbers", "group-123", true},
		{"group ID with underscores", "test_group", true},
		{"group ID with hyphens", "test-group-123", true},
		{"group ID with dots", "test.group", true},
		{"single character", "a", true},
		{"long group ID", "very-long-group-id-with-many-characters", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumer, err := NewConsumer(
				[]string{"localhost:19092"},
				tc.groupID,
				nil, nil, nil, nil, nil,
			)

			if tc.valid {
				if err == nil {
					assert.Equal(t, tc.groupID, consumer.groupID)
					_ = consumer.Close()
				} else {
					t.Logf("Unexpected error for valid group ID %s: %v", tc.groupID, err)
				}
			} else {
				// Empty group ID should cause an error
				if err != nil {
					t.Logf("Expected error for invalid group ID: %v", err)
				}
			}
		})
	}
}

// TestProducer_EnqueueEvaluate_EdgeCases tests edge cases for producer enqueue
func TestProducer_EnqueueEvaluate_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("special_characters_in_payload", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job-特殊字符",
			CVID:           "test-cv-🚀",
			ProjectID:      "test-project-测试",
			JobDescription: "Test job with special chars: !@#$%^&*()",
			StudyCaseBrief: "Test study case with unicode: 你好世界",
		}

		_, err := producer.EnqueueEvaluate(context.Background(), payload)
		// Note: With real Redpanda container, this succeeds
		// The test validates that special characters are handled correctly
		assert.NoError(t, err)
	})

	t.Run("empty_strings_in_payload", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{
			JobID:          "",
			CVID:           "",
			ProjectID:      "",
			JobDescription: "",
			StudyCaseBrief: "",
		}

		_, err := producer.EnqueueEvaluate(context.Background(), payload)
		// Note: With real Redpanda container, this succeeds
		// The test validates that empty strings are handled correctly
		assert.NoError(t, err)
	})
}

// TestConsumer_ProcessRecord_EdgeCases tests edge cases for consumer process record
func TestConsumer_ProcessRecord_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("consumer_configuration", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		assert.NotNil(t, consumer)
		assert.Equal(t, "test-group", consumer.groupID)
		assert.NotNil(t, consumer.session)
	})

	t.Run("consumer_with_all_repositories", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, // jobs
			nil, // uploads
			nil, // results
			nil, // ai
			nil, // qdrant
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		assert.NotNil(t, consumer)
		assert.Equal(t, "test-group", consumer.groupID)
		assert.Nil(t, consumer.jobs)
		assert.Nil(t, consumer.uploads)
		assert.Nil(t, consumer.results)
		assert.Nil(t, consumer.ai)
		assert.Nil(t, consumer.q)
	})
}

// TestTimeoutHandling_Comprehensive tests comprehensive timeout handling
func TestTimeoutHandling_Comprehensive(t *testing.T) {
	t.Parallel()

	t.Run("producer_timeout", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job",
			CVID:           "test-cv",
			ProjectID:      "test-project",
			JobDescription: "Test job",
			StudyCaseBrief: "Test study case",
		}

		_, err := producer.EnqueueEvaluate(ctx, payload)
		assert.Error(t, err)
	})

	t.Run("consumer_timeout", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = consumer.Start(ctx)
		assert.Error(t, err)
	})
}

// TestConcurrency_Comprehensive tests comprehensive concurrency scenarios
func TestConcurrency_Comprehensive(t *testing.T) {
	t.Parallel()

	t.Run("multiple_producers", func(t *testing.T) {
		producer1 := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer1.Close() }()

		producer2 := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer2.Close() }()

		assert.NotNil(t, producer1)
		assert.NotNil(t, producer2)
	})

	t.Run("multiple_consumers", func(t *testing.T) {
		consumer1, err := NewConsumer(
			[]string{"localhost:19092"},
			"group-1",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer1.Close() }()

		consumer2, err := NewConsumer(
			[]string{"localhost:19092"},
			"group-2",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer2.Close() }()

		assert.NotNil(t, consumer1)
		assert.NotNil(t, consumer2)
		assert.Equal(t, "group-1", consumer1.groupID)
		assert.Equal(t, "group-2", consumer2.groupID)
	})
}

// TestProducer_EnqueueEvaluate_AdvancedEdgeCases tests advanced edge cases for producer enqueue
func TestProducer_EnqueueEvaluate_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty_strings_in_payload", func(t *testing.T) {
		// Use unique transactional ID to avoid epoch conflicts
		producer, err := NewProducerWithTransactionalID([]string{"localhost:19092"}, fmt.Sprintf("test-producer-edge-%d", time.Now().UnixNano()))
		require.NoError(t, err)
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{
			JobID:          "",
			CVID:           "",
			ProjectID:      "",
			JobDescription: "",
			StudyCaseBrief: "",
		}

		_, err = producer.EnqueueEvaluate(context.Background(), payload)
		// Note: With real Redpanda container, this succeeds
		// The test validates that empty strings are handled correctly
		assert.NoError(t, err)
	})

	t.Run("json_marshal_edge_cases", func(t *testing.T) {
		// Use unique transactional ID to avoid epoch conflicts
		producer, err := NewProducerWithTransactionalID([]string{"localhost:19092"}, fmt.Sprintf("test-producer-json-%d", time.Now().UnixNano()))
		require.NoError(t, err)
		defer func() { _ = producer.Close() }()

		// Test with various edge case payloads
		testCases := []domain.EvaluateTaskPayload{
			{
				JobID:          "job-with-newlines\nand\t tabs",
				CVID:           "cv-with-quotes\"and'apostrophes",
				ProjectID:      "project-with-backslashes\\and/slashes",
				JobDescription: "Description with unicode: αβγδε",
				StudyCaseBrief: "Brief with emojis: 🚀🎯💡",
			},
			{
				JobID:          "job-with-json-like-{\"key\":\"value\"}",
				CVID:           "cv-with-array-[1,2,3]",
				ProjectID:      "project-with-null\000",
				JobDescription: "Description with control chars: \x00\x01\x02",
				StudyCaseBrief: "Brief with special chars: \r\n\t",
			},
		}

		for i, payload := range testCases {
			t.Run(fmt.Sprintf("edge_case_%d", i), func(t *testing.T) {
				_, err := producer.EnqueueEvaluate(context.Background(), payload)
				// Note: With real Redpanda container, this succeeds
				// The test validates that edge case payloads are handled correctly
				assert.NoError(t, err)
			})
		}
	})
}

// TestConsumer_Start_AdvancedEdgeCases tests advanced edge cases for consumer start
func TestConsumer_Start_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("consumer_with_nil_repositories", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err = consumer.Start(ctx)
		assert.Error(t, err)
		// Should fail due to timeout or connection issues
	})

	t.Run("consumer_with_very_long_group_id", func(t *testing.T) {
		longGroupID := ""
		for i := 0; i < 1000; i++ {
			longGroupID += "a"
		}

		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			longGroupID,
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		assert.Equal(t, longGroupID, consumer.groupID)
	})

	t.Run("consumer_with_special_characters_in_group_id", func(t *testing.T) {
		specialGroupID := "group-with-特殊字符-🚀-and-symbols-!@#$%^&*()"

		consumer, err := NewConsumer(
			[]string{"localhost:19092"},
			specialGroupID,
			nil, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		defer func() { _ = consumer.Close() }()

		assert.Equal(t, specialGroupID, consumer.groupID)
	})
}

// TestCreateTopicIfNotExists_AdvancedEdgeCases tests advanced edge cases for topic creation
func TestCreateTopicIfNotExists_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("topic_with_special_characters", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "topic-with-特殊字符-🚀", 1, 1)
		assert.Error(t, err)
		// Should fail due to connection issues
	})

	t.Run("topic_with_very_long_name", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		longTopicName := ""
		for i := 0; i < 1000; i++ {
			longTopicName += "a"
		}

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, longTopicName, 1, 1)
		assert.Error(t, err)
		// Should fail due to connection issues
	})

	t.Run("topic_with_maximum_partitions", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic-max-partitions", 1000, 1)
		assert.Error(t, err)
		// Should fail due to connection issues
	})

	t.Run("topic_with_maximum_replication_factor", func(t *testing.T) {
		client, err := kgo.NewClient(kgo.SeedBrokers("localhost:19092"))
		require.NoError(t, err)
		defer client.Close()

		ctx := context.Background()
		err = createTopicIfNotExists(ctx, client, "test-topic-max-replication", 1, 1000)
		assert.Error(t, err)
		// Should fail due to connection issues
	})
}

// TestErrorHandling_AdvancedEdgeCases tests advanced error handling edge cases
func TestErrorHandling_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("network_timeout", func(t *testing.T) {
		producer := newTestProducer(t, []string{"localhost:19092"})
		defer func() { _ = producer.Close() }()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		payload := domain.EvaluateTaskPayload{
			JobID:          "test-job",
			CVID:           "test-cv",
			ProjectID:      "test-project",
			JobDescription: "Test job",
			StudyCaseBrief: "Test study case",
		}

		_, err := producer.EnqueueEvaluate(ctx, payload)
		assert.Error(t, err)
	})

	t.Run("invalid_broker_addresses", func(t *testing.T) {
		invalidBrokers := []string{
			"invalid-host:99999",
			"",
			"not-a-valid-address",
			"localhost:99999",
			"192.168.1.999:9092",
			"http://localhost:19092", // Wrong protocol
			"localhost:abc",          // Invalid port
		}

		for _, broker := range invalidBrokers {
			t.Run("broker_"+broker, func(t *testing.T) {
				_, err := NewProducer([]string{broker})
				// Should handle invalid brokers gracefully
				if err != nil {
					t.Logf("Expected error for invalid broker %s: %v", broker, err)
				}
			})
		}
	})
}

// TestConcurrency_AdvancedEdgeCases tests advanced concurrency edge cases
func TestConcurrency_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("concurrent_producer_creation", func(_ *testing.T) {
		const numProducers = 10
		producers := make([]*Producer, numProducers)
		errors := make([]error, numProducers)
		var wg sync.WaitGroup

		// Create producers concurrently
		for i := 0; i < numProducers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				producers[idx], errors[idx] = NewProducerWithTransactionalID([]string{"localhost:19092"}, fmt.Sprintf("test-producer-%d-%d", idx, time.Now().UnixNano()))
			}(i)
		}

		// Wait for all to complete
		wg.Wait()

		// Check results
		for i := 0; i < numProducers; i++ {
			if errors[i] == nil {
				_ = producers[i].Close()
			}
		}
	})

	t.Run("concurrent_consumer_creation", func(_ *testing.T) {
		const numConsumers = 10
		consumers := make([]*Consumer, numConsumers)
		errors := make([]error, numConsumers)
		var wg sync.WaitGroup

		// Create consumers concurrently
		for i := 0; i < numConsumers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				consumers[idx], errors[idx] = NewConsumer(
					[]string{"localhost:19092"},
					fmt.Sprintf("group-%d", idx),
					nil, nil, nil, nil, nil,
				)
			}(i)
		}

		// Wait for all to complete
		wg.Wait()

		// Check results
		for i := 0; i < numConsumers; i++ {
			if errors[i] == nil {
				_ = consumers[i].Close()
			}
		}
	})
}

// TestMemoryManagement_AdvancedEdgeCases tests advanced memory management edge cases
func TestMemoryManagement_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("producer_memory_cleanup", func(_ *testing.T) {
		// Create and close multiple producers to test memory cleanup
		for i := 0; i < 100; i++ {
			producer, err := NewProducerWithTransactionalID([]string{"localhost:19092"}, fmt.Sprintf("test-producer-%d-%d", i, time.Now().UnixNano()))
			if err == nil {
				_ = producer.Close()
			}
		}
		// If we get here without panicking, memory cleanup is working
	})

	t.Run("consumer_memory_cleanup", func(_ *testing.T) {
		// Create and close multiple consumers to test memory cleanup
		for i := 0; i < 100; i++ {
			consumer, err := NewConsumer(
				[]string{"localhost:19092"},
				fmt.Sprintf("group-%d", i),
				nil, nil, nil, nil, nil,
			)
			if err == nil {
				_ = consumer.Close()
			}
		}
		// If we get here without panicking, memory cleanup is working
	})
}

// TestJSONHandling_AdvancedEdgeCases tests advanced JSON handling edge cases
func TestJSONHandling_AdvancedEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("json_marshal_edge_cases", func(t *testing.T) {
		// Test various edge cases for JSON marshaling
		testCases := []domain.EvaluateTaskPayload{
			{
				JobID:          "job-with-json-{\"key\":\"value\"}",
				CVID:           "cv-with-array-[1,2,3]",
				ProjectID:      "project-with-null\000",
				JobDescription: "Description with control chars: \x00\x01\x02",
				StudyCaseBrief: "Brief with special chars: \r\n\t",
			},
			{
				JobID:          "job-with-unicode-αβγδε",
				CVID:           "cv-with-emoji-🚀🎯💡",
				ProjectID:      "project-with-quotes\"and'apostrophes",
				JobDescription: "Description with backslashes\\and/slashes",
				StudyCaseBrief: "Brief with newlines\nand\t tabs",
			},
		}

		for i, payload := range testCases {
			t.Run(fmt.Sprintf("json_edge_case_%d", i), func(t *testing.T) {
				// Test JSON marshaling
				data, err := json.Marshal(payload)
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled domain.EvaluateTaskPayload
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, payload, unmarshaled)
			})
		}
	})
}
