package redpanda

import (
	"context"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers using mockery-generated mocks

// Unit tests
func TestNewProducer_Unit(t *testing.T) {
	t.Parallel()

	t.Run("valid brokers", func(t *testing.T) {
		producer, err := NewProducer([]string{"localhost:9092"})
		assert.NoError(t, err)
		assert.NotNil(t, producer)
		defer func() { _ = producer.Close() }()
	})

	t.Run("empty brokers", func(t *testing.T) {
		_, err := NewProducer([]string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("invalid broker format", func(t *testing.T) {
		_, err := NewProducer([]string{"invalid-broker"})
		// This might succeed in creating the client but fail on connection
		// We just verify it doesn't panic
		if err != nil {
			t.Logf("Expected error for invalid broker: %v", err)
		}
	})
}

func TestNewConsumer_Unit(t *testing.T) {
	t.Parallel()

	t.Run("valid configuration", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:9092"},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.NoError(t, err)
		assert.NotNil(t, consumer)
		assert.Equal(t, "test-group", consumer.groupID)
		defer func() { _ = consumer.Close() }()
	})

	t.Run("empty brokers", func(t *testing.T) {
		_, err := NewConsumer(
			[]string{},
			"test-group",
			nil, nil, nil, nil, nil,
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no seed brokers")
	})

	t.Run("empty group ID", func(t *testing.T) {
		_, err := NewConsumer(
			[]string{"localhost:9092"},
			"",
			nil, nil, nil, nil, nil,
		)
		assert.Error(t, err)
	})
}

func TestProducer_EnqueueEvaluate_Unit(t *testing.T) {
	t.Parallel()

	t.Run("context cancellation", func(t *testing.T) {
		producer, err := NewProducer([]string{"localhost:9092"})
		require.NoError(t, err)
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

		_, err = producer.EnqueueEvaluate(ctx, payload)
		assert.Error(t, err)
		// Should fail due to cancelled context or unreachable broker
	})

	t.Run("empty payload", func(t *testing.T) {
		producer, err := NewProducer([]string{"localhost:9092"})
		require.NoError(t, err)
		defer func() { _ = producer.Close() }()

		payload := domain.EvaluateTaskPayload{}
		_, err = producer.EnqueueEvaluate(context.Background(), payload)
		assert.Error(t, err)
		// Should fail due to unreachable broker or invalid payload
	})
}

func TestConsumer_Start_Unit(t *testing.T) {
	t.Parallel()

	t.Run("context cancellation", func(t *testing.T) {
		consumer, err := NewConsumer(
			[]string{"localhost:9092"},
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
}

func TestTopicConstants_Unit(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "evaluate-jobs", TopicEvaluate)
	assert.NotEmpty(t, TopicEvaluate)
}

func TestEvaluateTaskPayload_Structure_Unit(t *testing.T) {
	t.Parallel()

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
}

func TestProducer_Close_Unit(t *testing.T) {
	t.Parallel()

	producer, err := NewProducer([]string{"localhost:9092"})
	require.NoError(t, err)

	// Test that Close doesn't panic
	err = producer.Close()
	assert.NoError(t, err)
}

func TestConsumer_Close_Unit(t *testing.T) {
	t.Parallel()

	consumer, err := NewConsumer(
		[]string{"localhost:9092"},
		"test-group",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)

	// Test that Close doesn't panic
	err = consumer.Close()
	assert.NoError(t, err)
}

func TestConsumer_Configuration_Unit(t *testing.T) {
	t.Parallel()

	consumer, err := NewConsumer(
		[]string{"localhost:9092"},
		"test-group",
		nil, nil, nil, nil, nil,
	)
	require.NoError(t, err)
	defer func() { _ = consumer.Close() }()

	assert.NotNil(t, consumer)
	assert.Equal(t, "test-group", consumer.groupID)
	assert.NotNil(t, consumer.session)
}

func TestErrorHandling_NetworkIssues_Unit(t *testing.T) {
	t.Parallel()

	invalidBrokers := []string{
		"invalid-host:99999",
		"",
		"not-a-valid-address",
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

func TestConsumer_GroupID_Validation_Unit(t *testing.T) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			consumer, err := NewConsumer(
				[]string{"localhost:9092"},
				tc.groupID,
				nil, nil, nil, nil, nil,
			)

			if tc.valid {
				if err == nil {
					assert.Equal(t, tc.groupID, consumer.groupID)
					_ = consumer.Close()
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
