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
	"sync"
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
	topic   string
	// Dynamic worker pool configuration
	maxWorkers    int
	minWorkers    int
	workerPool    chan struct{}
	activeWorkers int
	workerMu      sync.RWMutex
	jobQueue      chan *kgo.Record
	shutdown      chan struct{}
}

// NewConsumer constructs a Consumer with exactly-once semantics.
func NewConsumer(brokers []string, groupID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client) (*Consumer, error) {
	return NewConsumerWithTransactionalID(brokers, groupID, "ai-cv-evaluator-consumer", jobs, uploads, results, aicl, qcli)
}

// NewConsumerWithTransactionalID constructs a Consumer with a custom transactional ID.
// This is useful for testing to avoid conflicts between multiple consumers.
func NewConsumerWithTransactionalID(brokers []string, groupID string, transactionalID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client) (*Consumer, error) {
	return NewConsumerWithConfig(brokers, groupID, transactionalID, jobs, uploads, results, aicl, qcli, 2, 10) // Default: 2-10 workers
}

// NewConsumerWithConfig constructs a Consumer with custom configuration.
func NewConsumerWithConfig(brokers []string, groupID string, transactionalID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client, minWorkers, maxWorkers int) (*Consumer, error) {
	return NewConsumerWithTopic(brokers, groupID, transactionalID, jobs, uploads, results, aicl, qcli, minWorkers, maxWorkers, TopicEvaluate)
}

// NewConsumerWithTopic constructs a Consumer with a custom topic.
// This method allows tests to use unique topics for isolation.
func NewConsumerWithTopic(brokers []string, groupID string, transactionalID string, jobs domain.JobRepository, uploads domain.UploadRepository, results domain.ResultRepository, aicl domain.AIClient, qcli *qdrantcli.Client, minWorkers, maxWorkers int, topic string) (*Consumer, error) {
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

	if err := createTopicIfNotExists(ctx, tempClient, topic, 1, 1); err != nil {
		slog.Warn("failed to create topic, it may already exist",
			slog.String("topic", topic),
			slog.Any("error", err))
		// Don't fail if topic creation fails - it might already exist
	}

	// Create transactional session for EOS semantics
	slog.Info("creating redpanda transactional session",
		slog.String("brokers", fmt.Sprintf("%v", brokers)),
		slog.String("transactional_id", transactionalID),
		slog.String("group_id", groupID),
		slog.String("topic", topic))

	session, err := kgo.NewGroupTransactSession(
		kgo.SeedBrokers(brokers...),
		kgo.TransactionalID(transactionalID),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.RequireStableFetchOffsets(),
	)
	if err != nil {
		slog.Error("failed to create redpanda transactional session",
			slog.Any("error", err),
			slog.String("brokers", fmt.Sprintf("%v", brokers)),
			slog.String("transactional_id", transactionalID),
			slog.String("group_id", groupID),
			slog.String("topic", topic))
		return nil, fmt.Errorf("redpanda transactional session: %w", err)
	}

	slog.Info("redpanda transactional session created successfully",
		slog.String("transactional_id", transactionalID),
		slog.String("group_id", groupID),
		slog.String("topic", topic))

	slog.Info("redpanda consumer created successfully", slog.Int("min_workers", minWorkers), slog.Int("max_workers", maxWorkers))
	return &Consumer{
		session:       session,
		jobs:          jobs,
		uploads:       uploads,
		results:       results,
		ai:            aicl,
		q:             qcli,
		groupID:       groupID,
		topic:         topic,
		minWorkers:    minWorkers,
		maxWorkers:    maxWorkers,
		workerPool:    make(chan struct{}, maxWorkers),
		jobQueue:      make(chan *kgo.Record, maxWorkers*2), // Buffer for job queue
		shutdown:      make(chan struct{}),
		activeWorkers: minWorkers,
	}, nil
}

// Start begins consuming messages from Redpanda with dynamic worker pool.
func (c *Consumer) Start(ctx context.Context) error {
	slog.Info("starting redpanda consumer with dynamic worker pool",
		slog.String("group_id", c.groupID),
		slog.String("topic", c.topic),
		slog.Int("min_workers", c.minWorkers),
		slog.Int("max_workers", c.maxWorkers))

	// Start the initial worker pool
	slog.Info("starting initial worker pool", slog.Int("workers", c.minWorkers))
	c.startWorkerPool(ctx)

	// Start the message fetcher
	slog.Info("starting message fetcher goroutine")
	go c.messageFetcher(ctx)

	// Start the worker pool manager for dynamic scaling
	slog.Info("starting worker pool manager goroutine")
	go c.workerPoolManager(ctx)

	// Wait for shutdown signal
	slog.Info("consumer started successfully, waiting for shutdown signal")
	<-ctx.Done()
	slog.Info("redpanda consumer shutting down due to context cancellation")
	close(c.shutdown)
	return ctx.Err()
}

// startWorkerPool starts the initial set of workers
func (c *Consumer) startWorkerPool(ctx context.Context) {
	for i := 0; i < c.minWorkers; i++ {
		go c.worker(ctx, i)
	}
	slog.Info("started initial worker pool", slog.Int("workers", c.minWorkers))
}

// workerPoolManager manages dynamic scaling of workers
func (c *Consumer) workerPoolManager(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.shutdown:
			return
		case <-ticker.C:
			c.scaleWorkers(ctx)
		}
	}
}

// scaleWorkers dynamically scales the worker pool based on queue length
func (c *Consumer) scaleWorkers(ctx context.Context) {
	queueLen := len(c.jobQueue)
	activeWorkers := c.getActiveWorkers()

	// Scale up if queue has jobs and we haven't reached max workers
	if queueLen > 0 && activeWorkers < c.maxWorkers {
		workersToAdd := minInt(queueLen, c.maxWorkers-activeWorkers)
		if workersToAdd > 0 {
			for i := 0; i < workersToAdd; i++ {
				go c.worker(ctx, activeWorkers+i)
			}
			slog.Info("scaled up workers", slog.Int("added", workersToAdd), slog.Int("queue_length", queueLen), slog.Int("total_active", activeWorkers+workersToAdd))
		}
	}

	// Log current status for debugging
	if queueLen > 0 {
		slog.Info("worker pool status", slog.Int("queue_length", queueLen), slog.Int("active_workers", activeWorkers), slog.Int("max_workers", c.maxWorkers))
	}
}

// messageFetcher fetches messages from Kafka and queues them for processing
func (c *Consumer) messageFetcher(ctx context.Context) {
	slog.Info("messageFetcher started", slog.String("topic", c.topic), slog.String("group_id", c.groupID))

	pollCount := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("messageFetcher shutting down due to context cancellation")
			return
		case <-c.shutdown:
			slog.Info("messageFetcher shutting down due to shutdown signal")
			return
		default:
			pollCount++
			slog.Info("messageFetcher polling for messages",
				slog.Int("poll_count", pollCount),
				slog.String("topic", c.topic),
				slog.String("group_id", c.groupID))

			// Add timeout to prevent hanging on connection issues
			fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			slog.Info("calling session.PollFetches", slog.String("topic", c.topic))
			fetches := c.session.PollFetches(fetchCtx)
			cancel()

			slog.Info("session.PollFetches completed",
				slog.Int("num_records", fetches.NumRecords()),
				slog.Int("num_errors", len(fetches.Errors())))

			if errs := fetches.Errors(); len(errs) > 0 {
				slog.Error("fetch errors detected", slog.Int("error_count", len(errs)))
				for i, err := range errs {
					slog.Error("fetch error details",
						slog.Int("error_index", i),
						slog.Any("error", err),
						slog.String("topic", err.Topic),
						slog.Int("partition", int(err.Partition)),
						slog.String("error_type", fmt.Sprintf("%T", err.Err)),
						slog.String("error_message", func() string {
							if err.Err != nil {
								return err.Err.Error()
							}
							return "nil error"
						}()))

					// Only return on fatal connection errors, not on timeout or temporary issues
					if err.Err != nil && (err.Err.Error() == "unable to dial" || err.Err.Error() == "context canceled") {
						slog.Error("fatal connection error detected, shutting down messageFetcher")
						return
					}
				}
				// Continue polling on timeout or temporary errors
				slog.Info("continuing polling after errors", slog.Duration("sleep_duration", 2*time.Second))
				time.Sleep(2 * time.Second)
				continue
			}

			// If no records to process, continue polling
			if fetches.NumRecords() == 0 {
				// Log every 30 seconds that we're waiting for messages
				if time.Now().Unix()%30 == 0 {
					slog.Info("consumer waiting for messages",
						slog.String("topic", c.topic),
						slog.String("group_id", c.groupID),
						slog.Int("poll_count", pollCount))
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Queue all records for processing
			fetches.EachRecord(func(record *kgo.Record) {
				select {
				case c.jobQueue <- record:
					// Successfully queued
				default:
					// Queue is full, process synchronously
					slog.Warn("job queue full, processing synchronously", slog.Int64("offset", record.Offset))
					go func() { _ = c.processRecord(ctx, record) }()
				}
			})

			slog.Info("queued messages for processing", slog.Int("count", fetches.NumRecords()), slog.Int("queue_length", len(c.jobQueue)))
		}
	}
}

// worker processes jobs from the queue
func (c *Consumer) worker(ctx context.Context, workerID int) {
	slog.Info("worker started",
		slog.Int("worker_id", workerID),
		slog.String("topic", c.topic),
		slog.String("group_id", c.groupID))

	jobCount := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down due to context cancellation",
				slog.Int("worker_id", workerID),
				slog.Int("jobs_processed", jobCount))
			return
		case <-c.shutdown:
			slog.Info("worker shutting down due to shutdown signal",
				slog.Int("worker_id", workerID),
				slog.Int("jobs_processed", jobCount))
			return
		case record := <-c.jobQueue:
			// Check if record is nil (channel closed)
			if record == nil {
				slog.Info("worker received nil record, shutting down",
					slog.Int("worker_id", workerID),
					slog.Int("jobs_processed", jobCount))
				return
			}

			jobCount++
			slog.Info("worker received job from queue",
				slog.Int("worker_id", workerID),
				slog.Int("job_count", jobCount),
				slog.Int64("offset", record.Offset),
				slog.String("topic", record.Topic),
				slog.Int("partition", int(record.Partition)))

			// Increment active workers when processing starts
			c.incrementActiveWorkers()

			slog.Info("worker processing job",
				slog.Int("worker_id", workerID),
				slog.Int64("offset", record.Offset),
				slog.String("topic", record.Topic),
				slog.Int("partition", int(record.Partition)))

			if err := c.processRecord(ctx, record); err != nil {
				slog.Error("failed to process record",
					slog.Int("worker_id", workerID),
					slog.Int64("offset", record.Offset),
					slog.String("topic", record.Topic),
					slog.Int("partition", int(record.Partition)),
					slog.Any("error", err))
			} else {
				slog.Info("worker completed job successfully",
					slog.Int("worker_id", workerID),
					slog.Int64("offset", record.Offset),
					slog.String("topic", record.Topic),
					slog.Int("partition", int(record.Partition)))
			}

			// Decrement active workers when processing completes
			c.decrementActiveWorkers()
		}
	}
}

// Helper functions for worker management
func (c *Consumer) getActiveWorkers() int {
	c.workerMu.RLock()
	defer c.workerMu.RUnlock()
	return c.activeWorkers
}

func (c *Consumer) incrementActiveWorkers() {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	c.activeWorkers++
}

func (c *Consumer) decrementActiveWorkers() {
	c.workerMu.Lock()
	defer c.workerMu.Unlock()
	c.activeWorkers--
}

// Helper function for min
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// processRecord processes a single Kafka record with the evaluation logic.
func (c *Consumer) processRecord(ctx context.Context, record *kgo.Record) error {
	slog.Info("processRecord started",
		slog.String("topic", record.Topic),
		slog.Int64("offset", record.Offset),
		slog.Int("partition", int(record.Partition)),
		slog.Int("value_length", len(record.Value)))

	tracer := otel.Tracer("queue.consumer")
	ctx, span := tracer.Start(ctx, "ProcessEvaluateJob")
	defer span.End()

	slog.Info("consumer received message",
		slog.String("topic", record.Topic),
		slog.Int64("offset", record.Offset),
		slog.Int("partition", int(record.Partition)),
		slog.String("key", string(record.Key)),
		slog.Int("value_size", len(record.Value)))

	var payload domain.EvaluateTaskPayload
	slog.Info("attempting to unmarshal payload", slog.Int("value_size", len(record.Value)))
	if err := json.Unmarshal(record.Value, &payload); err != nil {
		slog.Error("failed to unmarshal payload",
			slog.Any("error", err),
			slog.String("value_preview", string(record.Value[:minInt(100, len(record.Value))])),
			slog.Int("value_length", len(record.Value)))
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	slog.Info("payload unmarshaled successfully",
		slog.String("job_id", payload.JobID),
		slog.String("cv_id", payload.CVID),
		slog.String("project_id", payload.ProjectID))

	slog.Info("processing evaluate task",
		slog.String("job_id", payload.JobID),
		slog.String("cv_id", payload.CVID),
		slog.String("project_id", payload.ProjectID))

	// Call the shared evaluation handler (defaults: two-pass + chaining enabled)
	slog.Info("calling shared.HandleEvaluate", slog.String("job_id", payload.JobID))
	err := shared.HandleEvaluate(ctx, c.jobs, c.uploads, c.results, c.ai, c.q, payload)
	if err != nil {
		slog.Error("evaluate task failed",
			slog.String("job_id", payload.JobID),
			slog.Any("error", err))
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
	if c.shutdown != nil {
		select {
		case <-c.shutdown:
			// Channel already closed
		default:
			close(c.shutdown)
		}
	}
	if c.jobQueue != nil {
		select {
		case <-c.jobQueue:
			// Channel already closed
		default:
			close(c.jobQueue)
		}
	}
	return nil
}
