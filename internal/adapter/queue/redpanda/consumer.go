package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/queue/shared"
	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
)

// Consumer wraps a Kafka consumer with exactly-once processing semantics.
type Consumer struct {
	session *kgo.GroupTransactSession
	jobs    domain.JobRepository
	uploads domain.UploadRepository
	results domain.ResultRepository
	ai      domain.AIClient
	q       *qdrantcli.Client
	groupID string
}

// NewConsumer constructs a Consumer with exactly-once semantics.
func NewConsumer(brokers []string, groupID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client) (*Consumer, error) {
	return NewConsumerWithTransactionalID(brokers, groupID, "ai-cv-evaluator-consumer", jobs, uploads, results, aicl, qcli)
}

// NewConsumerWithTransactionalID constructs a Consumer with a custom transactional ID.
// This is useful for testing to avoid conflicts between multiple consumers.
func NewConsumerWithTransactionalID(brokers []string, groupID string, transactionalID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client) (*Consumer, error) {
	slog.Info("creating redpanda consumer", slog.Any("brokers", brokers), slog.String("group_id", groupID), slog.String("transactional_id", transactionalID))

	// Validate brokers
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no seed brokers provided")
	}

	// Validate group ID
	if groupID == "" {
		return nil, fmt.Errorf("missing required group ID")
	}

	// Create topic if it doesn't exist first
	ctx := context.Background()
	tempClient, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		slog.Error("failed to create temp client for topic creation", slog.Any("error", err))
		return nil, fmt.Errorf("temp client: %w", err)
	}
	defer tempClient.Close()

	if err := createTopicIfNotExists(ctx, tempClient, TopicEvaluate, 1, 1); err != nil {
		slog.Warn("failed to create topic, it may already exist",
			slog.String("topic", TopicEvaluate),
			slog.Any("error", err))
		// Don't fail if topic creation fails - it might already exist
	}

	// Create transactional session for EOS semantics
	session, err := kgo.NewGroupTransactSession(
		kgo.SeedBrokers(brokers...),
		kgo.TransactionalID(transactionalID),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(TopicEvaluate),
		kgo.RequireStableFetchOffsets(),
	)
	if err != nil {
		slog.Error("failed to create redpanda transactional session", slog.Any("error", err))
		return nil, fmt.Errorf("redpanda transactional session: %w", err)
	}

	slog.Info("redpanda consumer created successfully")
	return &Consumer{
		session: session,
		jobs:    jobs,
		uploads: uploads,
		results: results,
		ai:      aicl,
		q:       qcli,
		groupID: groupID,
	}, nil
}

// Start begins consuming messages from Redpanda.
func (c *Consumer) Start(ctx context.Context) error {
	slog.Info("starting redpanda consumer", slog.String("group_id", c.groupID), slog.String("topic", TopicEvaluate))

	for {
		select {
		case <-ctx.Done():
			slog.Info("redpanda consumer shutting down")
			return ctx.Err()
		default:
			// Add timeout to prevent hanging on connection issues
			fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			fetches := c.session.PollFetches(fetchCtx)
			cancel()

			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					slog.Error("fetch error", slog.Any("error", err))
					// If we get connection errors, return early to prevent hanging
					if err.Err != nil && (err.Err.Error() == "context deadline exceeded" ||
						err.Err.Error() == "unable to dial" ||
						err.Err.Error() == "context canceled") {
						return err.Err
					}
				}
				continue
			}

			// Begin transaction for EOS semantics
			if err := c.session.Begin(); err != nil {
				slog.Error("failed to begin transaction", slog.Any("error", err))
				// If we can't begin transaction due to connection issues, return error
				if err.Error() == "unable to dial" || err.Error() == "context canceled" {
					return err
				}
				continue
			}

			// Use AbortingFirstErrPromise for proper error handling
			e := kgo.AbortingFirstErrPromise(c.session.Client())

			// Process all records in the transaction
			fetches.EachRecord(func(record *kgo.Record) {
				if err := c.processRecord(ctx, record); err != nil {
					slog.Error("failed to process record", slog.Int64("offset", record.Offset), slog.Any("error", err))
					// Error will be captured by AbortingFirstErrPromise
				}
			})

			// End transaction - commit if no errors, abort if errors
			committed, err := c.session.End(ctx, e.Err() == nil)
			if err != nil {
				slog.Error("failed to end transaction", slog.Any("error", err))
				continue
			}

			if committed {
				slog.Info("transaction committed successfully")
			} else {
				slog.Info("transaction aborted due to errors")
			}
		}
	}
}

// processRecord processes a single Kafka record with the evaluation logic.
func (c *Consumer) processRecord(ctx context.Context, record *kgo.Record) error {
	tracer := otel.Tracer("queue.consumer")
	ctx, span := tracer.Start(ctx, "ProcessEvaluateJob")
	defer span.End()

	slog.Info("consumer received message", slog.String("topic", record.Topic), slog.Int64("offset", record.Offset), slog.Int("partition", int(record.Partition)))

	var payload domain.EvaluateTaskPayload
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		slog.Error("failed to unmarshal payload", slog.Any("error", err))
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	slog.Info("processing evaluate task", slog.String("job_id", payload.JobID), slog.String("cv_id", payload.CVID), slog.String("project_id", payload.ProjectID))

	// Call the shared evaluation handler (defaults: two-pass + chaining enabled)
	err := shared.HandleEvaluate(ctx, c.jobs, c.uploads, c.results, c.ai, c.q, payload)
	if err != nil {
		slog.Error("evaluate task failed", slog.String("job_id", payload.JobID), slog.Any("error", err))
		return err
	}

	slog.Info("evaluate task completed successfully", slog.String("job_id", payload.JobID))
	return nil
}

// Close closes the consumer.
func (c *Consumer) Close() error {
	if c.session != nil {
		c.session.Close()
	}
	return nil
}
