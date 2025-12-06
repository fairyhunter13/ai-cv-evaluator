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
	"strings"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/plugin/kotel"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/observability"
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

	retryManager *RetryManager

	// Observability components
	observableClient *observability.IntegratedObservableClient
	groupID          string
	topic            string
	// Dynamic worker pool configuration
	maxWorkers    int
	minWorkers    int
	workerPool    chan struct{}
	activeWorkers int
	workerMu      sync.RWMutex
	jobQueue      chan *kgo.Record

	// Phase 1 Algorithm: Adaptive Polling
	adaptivePoller *AdaptivePoller
	shutdown       chan struct{}

	// Connection management
	brokers         []string
	transactionalID string
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

	// Validate brokers BEFORE using brokers[0]
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no seed brokers provided")
	}

	// Validate group ID BEFORE creating any clients
	if groupID == "" {
		return nil, fmt.Errorf("missing required group ID")
	}

	// Create integrated observable client for queue operations (safe to access brokers[0])
	observableClient := observability.NewIntegratedObservableClient(
		observability.ConnectionTypeQueue,
		observability.OperationTypePoll,
		brokers[0], // Use first broker as endpoint
		"ai-cv-evaluator-worker",
		10*time.Second, // Base timeout
		1*time.Second,  // Min timeout
		60*time.Second, // Max timeout
	)

	// Create topic if it doesn't exist first
	ctx := context.Background()
	tempClient, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		slog.Error("failed to create temp client for topic creation", slog.Any("error", err))
		return nil, fmt.Errorf("temp client: %w", err)
	}
	defer tempClient.Close()

	// Create optimized topic for parallel processing
	partitions := int32(8) // Multiple partitions for parallel processing
	replicationFactor := int16(1)

	if err := createOptimizedTopicForParallelProcessing(ctx, tempClient, topic, partitions, replicationFactor); err != nil {
		slog.Warn("failed to create optimized topic, falling back to standard topic creation",
			slog.String("topic", topic),
			slog.Any("error", err))
		// Fallback to standard topic creation
		if err := createTopicIfNotExists(ctx, tempClient, topic, 1, 1); err != nil {
			slog.Warn("failed to create topic, it may already exist",
				slog.String("topic", topic),
				slog.Any("error", err))
			// Don't fail if topic creation fails - it might already exist
		}
	}

	// Create transactional session for EOS semantics
	slog.Info("creating redpanda transactional session",
		slog.String("brokers", fmt.Sprintf("%v", brokers)),
		slog.String("transactional_id", transactionalID),
		slog.String("group_id", groupID),
		slog.String("topic", topic))

	// Create OpenTelemetry tracer for Kafka instrumentation
	kotelTracer := kotel.NewTracer(
		kotel.TracerProvider(otel.GetTracerProvider()),
	)
	kotelService := kotel.NewKotel(
		kotel.WithTracer(kotelTracer),
	)

	// Configure consumer options for parallel processing
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.TransactionalID(transactionalID),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.RequireStableFetchOffsets(),

		// Add OpenTelemetry hooks for distributed tracing
		kgo.WithHooks(kotelService.Hooks()...),

		// ✅ FIXED: Optimized timeouts for better Redpanda connectivity
		kgo.DialTimeout(10 * time.Second),           // Further reduced for faster connection
		kgo.RequestTimeoutOverhead(5 * time.Second), // Reduced for faster requests
		kgo.RetryTimeout(30 * time.Second),          // Reduced retry timeout
		kgo.SessionTimeout(30 * time.Second),        // Increased session timeout for stability
		kgo.HeartbeatInterval(3 * time.Second),      // More frequent heartbeats
		kgo.RebalanceTimeout(10 * time.Second),      // Faster rebalancing

		// ✅ FIXED: Optimized fetch settings for better message consumption
		kgo.FetchMaxBytes(10 * 1024 * 1024),         // Reduced from 50MB to 10MB
		kgo.FetchMaxWait(10 * time.Second),          // Increased to 10s for better stability
		kgo.FetchMinBytes(512),                      // Reduced from 1KB to 512B
		kgo.FetchMaxPartitionBytes(2 * 1024 * 1024), // Reduced from 10MB to 2MB

		// ✅ ADDED: Enable automatic offset commits for better message processing
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(1 * time.Second), // Commit offsets every 1 second
	}

	session, err := kgo.NewGroupTransactSession(opts...)
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
		observableClient: observableClient,
		session:          session,
		jobs:             jobs,
		uploads:          uploads,
		results:          results,
		ai:               aicl,
		q:                qcli,
		groupID:          groupID,
		topic:            topic,
		minWorkers:       minWorkers,
		maxWorkers:       maxWorkers,
		workerPool:       make(chan struct{}, maxWorkers),
		jobQueue:         make(chan *kgo.Record, maxWorkers*2), // Buffer for job queue
		shutdown:         make(chan struct{}),
		activeWorkers:    minWorkers,
		brokers:          brokers,
		transactionalID:  transactionalID,

		// Phase 1 Algorithm: Initialize adaptive poller
		adaptivePoller: NewAdaptivePoller(100 * time.Millisecond), // Start with 100ms base interval
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
	ticker := time.NewTicker(2 * time.Second) // Check every 2 seconds for faster scaling
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

	// Scale up: Add workers when queue has jobs and we have capacity
	if queueLen > 0 && activeWorkers < c.maxWorkers {
		workersToAdd := minInt(queueLen, c.maxWorkers-activeWorkers)
		if workersToAdd > 0 {
			// FIXED: Track workers properly and ensure we don't exceed max
			for i := 0; i < workersToAdd; i++ {
				// Check again before creating each worker to prevent race conditions
				if c.getActiveWorkers() < c.maxWorkers {
					// Increment active workers before starting the worker
					c.incrementActiveWorkers()
					go c.worker(ctx, c.getActiveWorkers())
				}
			}
			slog.Info("scaled up workers", slog.Int("added", workersToAdd), slog.Int("queue_length", queueLen), slog.Int("total_active", c.getActiveWorkers()))
		}
	}

	// Scale down: Remove workers when we have excess capacity
	// Scale down if we have more than min workers AND (queue is empty OR we have excess workers)
	if activeWorkers > c.minWorkers && (queueLen == 0 || activeWorkers > queueLen) {
		// Calculate how many workers we can remove
		// Keep at least min workers, but don't remove more than we have excess
		workersToRemove := activeWorkers - c.minWorkers
		if queueLen > 0 && activeWorkers > queueLen {
			// If we have more workers than jobs, remove excess workers
			workersToRemove = minInt(workersToRemove, activeWorkers-queueLen)
		}

		if workersToRemove > 0 {
			// Signal workers to stop gracefully
			for i := 0; i < workersToRemove; i++ {
				if c.getActiveWorkers() > c.minWorkers {
					c.decrementActiveWorkers()
					// Workers will check active count and exit gracefully
				}
			}
			slog.Info("scaled down workers", slog.Int("removed", workersToRemove), slog.Int("queue_length", queueLen), slog.Int("total_active", c.getActiveWorkers()))
		}
	}

	// Log current status for debugging
	if queueLen > 0 || activeWorkers > c.minWorkers {
		slog.Info("worker pool status", slog.Int("queue_length", queueLen), slog.Int("active_workers", activeWorkers), slog.Int("min_workers", c.minWorkers), slog.Int("max_workers", c.maxWorkers))
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

			// Phase 1 Algorithm: Use adaptive polling interval
			nextInterval := c.adaptivePoller.GetNextInterval()
			slog.Info("messageFetcher polling for messages",
				slog.Int("poll_count", pollCount),
				slog.String("topic", c.topic),
				slog.String("group_id", c.groupID),
				slog.Duration("adaptive_interval", nextInterval))

			// Use observable client with adaptive timeout and connection health check
			var fetches kgo.Fetches
			err := c.observableClient.ExecuteWithMetrics(ctx, "poll_fetches", func(fetchCtx context.Context) error {
				// Add connection health check before polling
				if !c.isConnectionHealthy() {
					slog.Warn("connection unhealthy, attempting to reconnect")
					// Try to re-establish connection
					if err := c.reconnectToRedpanda(); err != nil {
						slog.Error("failed to reconnect to Redpanda", slog.Any("error", err))
						return fmt.Errorf("connection unhealthy: %w", err)
					}
				}

				fetches = c.session.PollFetches(fetchCtx)
				return nil
			})

			if err != nil {
				slog.Error("poll fetches failed with observable metrics", slog.Any("error", err))
				// Add exponential backoff for connection errors
				backoffDuration := nextInterval
				if strings.Contains(err.Error(), "context deadline exceeded") ||
					strings.Contains(err.Error(), "connection") ||
					strings.Contains(err.Error(), "timeout") {
					slog.Warn("connection error detected, using exponential backoff", slog.Any("error", err))
					backoffDuration = time.Duration(pollCount) * 2 * time.Second
					if backoffDuration > 10*time.Second {
						backoffDuration = 10 * time.Second
					}
				}
				c.adaptivePoller.RecordFailure()
				time.Sleep(backoffDuration)
				continue
			}

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
				// FIXED: Better error recovery with exponential backoff for context deadline exceeded
				backoffDuration := 2 * time.Second
				for _, err := range errs {
					if err.Err != nil && err.Err.Error() == "context deadline exceeded" {
						// Exponential backoff for context deadline exceeded errors
						backoffDuration = time.Duration(pollCount) * 2 * time.Second
						if backoffDuration > 10*time.Second {
							backoffDuration = 10 * time.Second
						}
						slog.Warn("context deadline exceeded, using exponential backoff",
							slog.Duration("backoff_duration", backoffDuration),
							slog.Int("poll_count", pollCount))
						break
					}
				}
				// Phase 1 Algorithm: Record failed poll
				c.adaptivePoller.RecordFailure()

				slog.Info("continuing polling after errors", slog.Duration("sleep_duration", backoffDuration))
				time.Sleep(backoffDuration)
				continue
			}

			// If no records to process, continue polling
			if fetches.NumRecords() == 0 {
				// Phase 1 Algorithm: Record successful poll (no errors, just no messages)
				c.adaptivePoller.RecordSuccess()

				// Log every 30 seconds that we're waiting for messages
				if time.Now().Unix()%30 == 0 {
					slog.Info("consumer waiting for messages",
						slog.String("topic", c.topic),
						slog.String("group_id", c.groupID),
						slog.Int("poll_count", pollCount))
				}

				// Phase 1 Algorithm: Use adaptive sleep interval
				time.Sleep(nextInterval)
				continue
			}

			// Phase 1 Algorithm: Record successful poll (messages found)
			c.adaptivePoller.RecordSuccess()

			// Queue all records for processing
			fetches.EachRecord(func(record *kgo.Record) {
				jobID := string(record.Key)
				for _, h := range record.Headers {
					if h.Key == "job_id" {
						jobID = string(h.Value)
						break
					}
				}

				select {
				case c.jobQueue <- record:
					slog.Info("queued job for processing",
						slog.String("job_id", jobID),
						slog.Int64("offset", record.Offset),
						slog.String("topic", record.Topic),
						slog.Int("partition", int(record.Partition)),
						slog.Int("queue_length", len(c.jobQueue)))
				default:
					// Queue is full, process synchronously
					slog.Warn("job queue full, processing synchronously",
						slog.String("job_id", jobID),
						slog.Int64("offset", record.Offset),
						slog.String("topic", record.Topic),
						slog.Int("partition", int(record.Partition)))
					go func(rec *kgo.Record, _ string) { _ = c.processRecord(ctx, rec) }(record, jobID)
				}
			})

			slog.Info("queued messages for processing",
				slog.Int("count", fetches.NumRecords()),
				slog.Int("queue_length", len(c.jobQueue)))
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

			// FIXED: Don't increment/decrement active workers here
			// The worker is already active, we just process the job
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

			// Check if we should scale down after processing a job
			// If we have excess workers (more than min or more than queue length), exit gracefully
			activeWorkers := c.getActiveWorkers()
			queueLen := len(c.jobQueue)
			if activeWorkers > c.minWorkers && (queueLen == 0 || activeWorkers > queueLen) {
				slog.Info("worker scaling down due to excess capacity",
					slog.Int("worker_id", workerID),
					slog.Int("jobs_processed", jobCount),
					slog.Int("active_workers", activeWorkers),
					slog.Int("min_workers", c.minWorkers),
					slog.Int("queue_length", queueLen))
				return
			}
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
	if c.activeWorkers > 0 {
		c.activeWorkers--
	}
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

	// Attach request-scoped metadata to the worker context so that all
	// downstream logs (including AI client logs) are correlated by request_id.
	if payload.RequestID != "" {
		ctx = observability.ContextWithRequestID(ctx, payload.RequestID)
	}
	lg := observability.LoggerFromContext(ctx).With(
		slog.String("job_id", payload.JobID),
		slog.String("cv_id", payload.CVID),
		slog.String("project_id", payload.ProjectID),
	)
	if payload.RequestID != "" {
		lg = lg.With(slog.String("request_id", payload.RequestID))
	}
	ctx = observability.ContextWithLogger(ctx, lg)

	lg.Info("payload unmarshaled successfully")
	lg.Info("processing evaluate task")

	// Call the local evaluation handler (defaults: two-pass + chaining enabled)
	lg.Info("calling HandleEvaluate")
	err := HandleEvaluate(ctx, c.jobs, c.uploads, c.results, c.ai, c.q, payload)
	if err != nil {
		lg.Error("evaluate task failed", slog.Any("error", err))

		// If a retry manager is configured, route retryable upstream failures
		// (rate limits and timeouts) through the higher-level retry/DLQ flow.
		if c.retryManager != nil {
			code := classifyFailureCode(err.Error())
			if code == "UPSTREAM_RATE_LIMIT" || code == "UPSTREAM_TIMEOUT" {
				retryInfo := &domain.RetryInfo{
					AttemptCount:  0,
					LastAttemptAt: time.Now(),
					RetryStatus:   domain.RetryStatusNone,
					LastError:     err.Error(),
					ErrorHistory:  []string{err.Error()},
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				}
				if rErr := c.retryManager.RetryJob(ctx, payload.JobID, retryInfo, payload); rErr != nil {
					lg.Error("retry manager failed to handle job failure",
						slog.String("job_id", payload.JobID),
						slog.String("failure_code", code),
						slog.Any("error", rErr))
				} else {
					lg.Info("retry manager scheduled retry or moved job to DLQ",
						slog.String("job_id", payload.JobID),
						slog.String("failure_code", code))
				}
			}
		}
		return err
	}

	lg.Info("evaluate task completed successfully")
	return nil
}

// ...
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

// GetHealthStatus returns the health status of the consumer
func (c *Consumer) GetHealthStatus() map[string]interface{} {
	if c.observableClient == nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"reason": "observable client not initialized",
		}
	}

	healthStatus := c.observableClient.GetHealthStatus()
	healthStatus["consumer_type"] = "redpanda"
	healthStatus["group_id"] = c.groupID
	healthStatus["topic"] = c.topic
	healthStatus["active_workers"] = c.getActiveWorkers()
	healthStatus["min_workers"] = c.minWorkers
	healthStatus["max_workers"] = c.maxWorkers

	return healthStatus
}

// isConnectionHealthy checks if the connection to Redpanda is healthy
func (c *Consumer) isConnectionHealthy() bool {
	// Simple health check - try to get metadata
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if session is still valid
	if c.session == nil {
		return false
	}

	// Try a simple operation to check connectivity
	fetches := c.session.PollFetches(ctx)
	return len(fetches.Errors()) == 0
}

// reconnectToRedpanda attempts to reconnect to Redpanda
func (c *Consumer) reconnectToRedpanda() error {
	slog.Info("attempting to reconnect to Redpanda")

	// Close existing session
	if c.session != nil {
		c.session.Close()
	}

	// Recreate session with same configuration
	opts := []kgo.Opt{
		kgo.SeedBrokers(c.brokers...),
		kgo.TransactionalID(c.transactionalID),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.ConsumerGroup(c.groupID),
		kgo.ConsumeTopics(c.topic),
		kgo.RequireStableFetchOffsets(),

		// Optimized timeouts for better connectivity
		kgo.DialTimeout(10 * time.Second),
		kgo.RequestTimeoutOverhead(5 * time.Second),
		kgo.RetryTimeout(30 * time.Second),
		kgo.SessionTimeout(20 * time.Second),
		kgo.HeartbeatInterval(3 * time.Second),
		kgo.RebalanceTimeout(10 * time.Second),

		// Optimized fetch settings
		kgo.FetchMaxBytes(10 * 1024 * 1024),
		kgo.FetchMaxWait(2 * time.Second),
		kgo.FetchMinBytes(512),
		kgo.FetchMaxPartitionBytes(2 * 1024 * 1024),

		// Enable automatic offset commits
		kgo.AutoCommitMarks(),
		kgo.AutoCommitInterval(1 * time.Second),
	}

	session, err := kgo.NewGroupTransactSession(opts...)
	if err != nil {
		return fmt.Errorf("failed to recreate Redpanda session: %w", err)
	}

	c.session = session
	slog.Info("successfully reconnected to Redpanda")
	return nil
}

// IsHealthy returns true if the consumer is healthy
func (c *Consumer) IsHealthy() bool {
	if c.observableClient == nil {
		return false
	}
	return c.observableClient.IsHealthy()
}

// WithRetryManager attaches a RetryManager to the consumer for handling
// retryable failures via the retry/DLQ flow. When nil, the consumer behaves
// as before and simply returns the evaluation error.
func (c *Consumer) WithRetryManager(rm *RetryManager) *Consumer {
	c.retryManager = rm
	return c
}
