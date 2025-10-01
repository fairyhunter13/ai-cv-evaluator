// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
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
	// Channel-based approach for concurrent processing
	transactionChan chan struct{}
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
	return &Producer{
		client:          client,
		transactionChan: make(chan struct{}, 1), // Buffered channel for serializing transactions
	}, nil
}

// EnqueueEvaluate enqueues an evaluation task with exactly-once semantics.
func (p *Producer) EnqueueEvaluate(ctx domain.Context, payload domain.EvaluateTaskPayload) (string, error) {
	return p.EnqueueEvaluateToTopic(ctx, payload, TopicEvaluate)
}

// EnqueueEvaluateToTopic enqueues an evaluation task to a specific topic.
// This method allows tests to use unique topics for isolation.
func (p *Producer) EnqueueEvaluateToTopic(ctx domain.Context, payload domain.EvaluateTaskPayload, topic string) (string, error) {
	slog.Info("enqueueing evaluate task",
		slog.String("job_id", payload.JobID),
		slog.String("cv_id", payload.CVID),
		slog.String("project_id", payload.ProjectID),
		slog.String("topic", topic))

	// Use channel-based synchronization to serialize transactions
	// This allows concurrent processing while maintaining transaction safety
	slog.Info("acquiring transaction lock", slog.String("job_id", payload.JobID))
	select {
	case p.transactionChan <- struct{}{}:
		// Acquired transaction lock
		slog.Info("transaction lock acquired", slog.String("job_id", payload.JobID))
		defer func() {
			<-p.transactionChan
			slog.Info("transaction lock released", slog.String("job_id", payload.JobID))
		}() // Release lock when done
	case <-ctx.Done():
		slog.Error("context cancelled while acquiring transaction lock", slog.String("job_id", payload.JobID))
		return "", ctx.Err()
	}

	// Begin transaction for EOS semantics
	slog.Info("beginning transaction", slog.String("job_id", payload.JobID))
	if err := p.client.BeginTransaction(); err != nil {
		slog.Error("failed to begin transaction",
			slog.String("job_id", payload.JobID),
			slog.Any("error", err))
		return "", fmt.Errorf("begin transaction: %w", err)
	}
	slog.Info("transaction begun successfully", slog.String("job_id", payload.JobID))

	slog.Info("marshaling payload", slog.String("job_id", payload.JobID))
	b, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal payload",
			slog.String("job_id", payload.JobID),
			slog.Any("error", err))
		// Abort transaction on error
		slog.Info("aborting transaction due to marshal error", slog.String("job_id", payload.JobID))
		if abortErr := p.client.EndTransaction(ctx, kgo.TryAbort); abortErr != nil {
			slog.Error("failed to abort transaction", slog.Any("error", abortErr))
		}
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	slog.Info("payload marshaled successfully",
		slog.String("job_id", payload.JobID),
		slog.Int("payload_size", len(b)))

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(payload.JobID), // Use job ID as key for ordering
		Value: b,
		Headers: []kgo.RecordHeader{
			{Key: "job_id", Value: []byte(payload.JobID)},
			{Key: "cv_id", Value: []byte(payload.CVID)},
			{Key: "project_id", Value: []byte(payload.ProjectID)},
		},
	}

	// Use AbortingFirstErrPromise for proper error handling
	slog.Info("producing message to topic",
		slog.String("job_id", payload.JobID),
		slog.String("topic", topic))
	e := kgo.AbortingFirstErrPromise(p.client)
	p.client.Produce(ctx, record, e.Promise())

	// Check for production errors
	if err := e.Err(); err != nil {
		slog.Error("failed to produce message",
			slog.String("job_id", payload.JobID),
			slog.String("topic", topic),
			slog.Any("error", err))
		// Abort transaction on error
		slog.Info("aborting transaction due to produce error", slog.String("job_id", payload.JobID))
		if abortErr := p.client.EndTransaction(ctx, kgo.TryAbort); abortErr != nil {
			slog.Error("failed to abort transaction", slog.Any("error", abortErr))
		}
		return "", fmt.Errorf("produce: %w", err)
	}
	slog.Info("message produced successfully",
		slog.String("job_id", payload.JobID),
		slog.String("topic", topic))

	// Commit transaction for EOS semantics
	slog.Info("committing transaction", slog.String("job_id", payload.JobID))
	if err := p.client.EndTransaction(ctx, kgo.TryCommit); err != nil {
		slog.Error("failed to commit transaction",
			slog.String("job_id", payload.JobID),
			slog.Any("error", err))
		return "", fmt.Errorf("commit transaction: %w", err)
	}
	slog.Info("transaction committed successfully", slog.String("job_id", payload.JobID))

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
	if p.transactionChan != nil {
		select {
		case <-p.transactionChan:
			// Channel already closed
		default:
			close(p.transactionChan)
		}
	}
	return nil
}
