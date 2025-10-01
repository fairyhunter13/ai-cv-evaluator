package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/observability"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

const (
	// TopicEvaluate is the Kafka topic for evaluation jobs
	TopicEvaluate = "evaluate-jobs"
)

// Producer wraps a Kafka producer and implements domain.Queue.
type Producer struct {
	client *kgo.Client
}

// NewProducer constructs a Producer with exactly-once semantics.
func NewProducer(brokers []string) (*Producer, error) {
	return NewProducerWithTransactionalID(brokers, "ai-cv-evaluator-producer")
}

// NewProducerWithTransactionalID constructs a Producer with a custom transactional ID.
// This is useful for testing to avoid conflicts between multiple producers.
func NewProducerWithTransactionalID(brokers []string, transactionalID string) (*Producer, error) {
	slog.Info("creating redpanda producer", slog.Any("brokers", brokers), slog.String("transactional_id", transactionalID))

	// Validate brokers
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no seed brokers provided")
	}

	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		// Enable transactional producer for EOS semantics
		kgo.TransactionalID(transactionalID),
		// Enable retries for reliability
		kgo.RequestRetries(10),
		// Producer batch configuration
		kgo.ProducerBatchMaxBytes(1000000),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		slog.Error("failed to create redpanda client", slog.Any("error", err))
		return nil, fmt.Errorf("redpanda client: %w", err)
	}

	// Create topic if it doesn't exist
	ctx := context.Background()
	if err := createTopicIfNotExists(ctx, client, TopicEvaluate, 1, 1); err != nil {
		slog.Warn("failed to create topic, it may already exist",
			slog.String("topic", TopicEvaluate),
			slog.Any("error", err))
		// Don't fail if topic creation fails - it might already exist
	}

	slog.Info("redpanda producer created successfully")
	return &Producer{client: client}, nil
}

// EnqueueEvaluate enqueues an evaluation task with exactly-once semantics.
func (p *Producer) EnqueueEvaluate(ctx domain.Context, payload domain.EvaluateTaskPayload) (string, error) {
	slog.Info("enqueueing evaluate task", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.String("project_id", payload.ProjectID))

	// Begin transaction for EOS semantics
	if err := p.client.BeginTransaction(); err != nil {
		slog.Error("failed to begin transaction", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return "", fmt.Errorf("begin transaction: %w", err)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload", slog.String("job_id", payload.JobID), slog.Any("error", err))
		// Abort transaction on error
		if abortErr := p.client.EndTransaction(ctx, kgo.TryAbort); abortErr != nil {
			slog.Error("failed to abort transaction", slog.Any("error", abortErr))
		}
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	record := &kgo.Record{
		Topic: TopicEvaluate,
		Key:   []byte(payload.JobID), // Use job ID as key for ordering
		Value: b,
		Headers: []kgo.RecordHeader{
			{Key: "job_id", Value: []byte(payload.JobID)},
			{Key: "cv_id", Value: []byte(payload.CVID)},
			{Key: "project_id", Value: []byte(payload.ProjectID)},
		},
	}

	// Use AbortingFirstErrPromise for proper error handling
	e := kgo.AbortingFirstErrPromise(p.client)
	p.client.Produce(ctx, record, e.Promise())

	// Check for production errors
	if err := e.Err(); err != nil {
		slog.Error("failed to produce message", slog.String("job_id", payload.JobID), slog.Any("error", err))
		// Abort transaction on error
		if abortErr := p.client.EndTransaction(ctx, kgo.TryAbort); abortErr != nil {
			slog.Error("failed to abort transaction", slog.Any("error", abortErr))
		}
		return "", fmt.Errorf("produce: %w", err)
	}

	// Commit transaction for EOS semantics
	if err := p.client.EndTransaction(ctx, kgo.TryCommit); err != nil {
		slog.Error("failed to commit transaction", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return "", fmt.Errorf("commit transaction: %w", err)
	}

	observability.EnqueueJob("evaluate")
	slog.Info("redpanda enqueue successful", slog.String("topic", TopicEvaluate), slog.String("job_id", payload.JobID))

	// Return job ID as task ID
	return payload.JobID, nil
}

// Close closes the producer.
func (p *Producer) Close() error {
	if p.client != nil {
		p.client.Close()
	}
	return nil
}
