// Package redpanda implements DLQ consumer for processing failed jobs.
package redpanda

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/twmb/franz-go/pkg/kgo"
)

// DLQConsumer handles processing of Dead Letter Queue jobs
type DLQConsumer struct {
	client       *kgo.Client
	retryManager *RetryManager
	jobs         domain.JobRepository
	groupID      string
	topic        string
	shutdown     chan struct{}
}

// NewDLQConsumer creates a new DLQ consumer
func NewDLQConsumer(brokers []string, groupID string, retryManager *RetryManager, jobs domain.JobRepository) (*DLQConsumer, error) {
	slog.Info("creating DLQ consumer", slog.Any("brokers", brokers), slog.String("group_id", groupID))

	// Validate brokers
	if len(brokers) == 0 {
		return nil, fmt.Errorf("no seed brokers provided")
	}

	// Validate group ID
	if groupID == "" {
		return nil, fmt.Errorf("missing required group ID")
	}

	// Configure consumer options for DLQ processing
	opts := []kgo.Opt{
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics("dlq-jobs"),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.RequireStableFetchOffsets(),
		// DLQ-specific settings
		kgo.FetchMaxBytes(1048576),               // 1MB fetch size
		kgo.FetchMaxWait(100 * time.Millisecond), // 100ms fetch wait
		kgo.FetchMinBytes(1),                     // Minimum bytes to fetch
		kgo.FetchMaxPartitionBytes(1048576),      // 1MB per partition
		// Connection timeout configurations
		kgo.DialTimeout(30 * time.Second),
		kgo.RequestTimeoutOverhead(10 * time.Second),
		kgo.RetryTimeout(60 * time.Second),
		kgo.SessionTimeout(30 * time.Second),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		slog.Error("failed to create DLQ consumer client", slog.Any("error", err))
		return nil, fmt.Errorf("DLQ consumer client: %w", err)
	}

	slog.Info("DLQ consumer created successfully", slog.String("group_id", groupID))
	return &DLQConsumer{
		client:       client,
		retryManager: retryManager,
		jobs:         jobs,
		groupID:      groupID,
		topic:        "dlq-jobs",
		shutdown:     make(chan struct{}),
	}, nil
}

// Start begins consuming DLQ messages
func (dc *DLQConsumer) Start(ctx context.Context) error {
	slog.Info("starting DLQ consumer", slog.String("group_id", dc.groupID), slog.String("topic", dc.topic))

	go dc.dlqMessageProcessor(ctx)

	slog.Info("DLQ consumer started successfully")
	return nil
}

// Stop stops the DLQ consumer
func (dc *DLQConsumer) Stop() {
	slog.Info("stopping DLQ consumer")
	close(dc.shutdown)
	dc.client.Close()
	slog.Info("DLQ consumer stopped")
}

// dlqMessageProcessor processes DLQ messages
func (dc *DLQConsumer) dlqMessageProcessor(ctx context.Context) {
	slog.Info("DLQ message processor started", slog.String("topic", dc.topic), slog.String("group_id", dc.groupID))

	for {
		select {
		case <-ctx.Done():
			slog.Info("DLQ message processor shutting down due to context cancellation")
			return
		case <-dc.shutdown:
			slog.Info("DLQ message processor shutting down due to shutdown signal")
			return
		default:
			// Poll for DLQ messages
			fetchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			fetches := dc.client.PollFetches(fetchCtx)
			cancel()

			if errs := fetches.Errors(); len(errs) > 0 {
				slog.Error("DLQ fetch errors detected", slog.Int("error_count", len(errs)))
				for i, err := range errs {
					slog.Error("DLQ fetch error details",
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
				}
				time.Sleep(2 * time.Second)
				continue
			}

			// If no records to process, continue polling
			if fetches.NumRecords() == 0 {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process DLQ records
			fetches.EachRecord(func(record *kgo.Record) {
				dc.processDLQRecord(ctx, record)
			})

			slog.Info("processed DLQ messages", slog.Int("count", fetches.NumRecords()))
		}
	}
}

// processDLQRecord processes a single DLQ record
func (dc *DLQConsumer) processDLQRecord(ctx context.Context, record *kgo.Record) {
	slog.Info("processing DLQ record",
		slog.String("topic", record.Topic),
		slog.Int("partition", int(record.Partition)),
		slog.Int64("offset", record.Offset),
		slog.String("key", string(record.Key)))

	// Parse DLQ message
	var dlqMessage map[string]interface{}
	if err := json.Unmarshal(record.Value, &dlqMessage); err != nil {
		slog.Error("failed to unmarshal DLQ message",
			slog.String("topic", record.Topic),
			slog.Int("partition", int(record.Partition)),
			slog.Int64("offset", record.Offset),
			slog.Any("error", err))
		return
	}

	// Extract job ID
	jobID, ok := dlqMessage["job_id"].(string)
	if !ok {
		slog.Error("DLQ message missing job_id",
			slog.String("topic", record.Topic),
			slog.Int("partition", int(record.Partition)),
			slog.Int64("offset", record.Offset))
		return
	}

	// Extract DLQ data
	dlqDataBytes, ok := dlqMessage["dlq_data"].([]byte)
	if !ok {
		slog.Error("DLQ message missing dlq_data",
			slog.String("job_id", jobID),
			slog.String("topic", record.Topic),
			slog.Int("partition", int(record.Partition)),
			slog.Int64("offset", record.Offset))
		return
	}

	// Parse DLQ job
	var dlqJob domain.DLQJob
	if err := json.Unmarshal(dlqDataBytes, &dlqJob); err != nil {
		slog.Error("failed to unmarshal DLQ job",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return
	}

	// Process DLQ job
	if err := dc.retryManager.ProcessDLQJob(ctx, dlqJob); err != nil {
		slog.Error("failed to process DLQ job",
			slog.String("job_id", jobID),
			slog.Any("error", err))
		return
	}

	slog.Info("DLQ job processed successfully",
		slog.String("job_id", jobID),
		slog.String("original_failure_reason", dlqJob.FailureReason))
}

// GetDLQStats returns DLQ statistics
func (dc *DLQConsumer) GetDLQStats(_ context.Context) (map[string]interface{}, error) {
	// This would typically query the DLQ topic for statistics
	// For now, return a placeholder
	return map[string]interface{}{
		"dlq_messages_processed":   0,
		"dlq_messages_failed":      0,
		"dlq_messages_reprocessed": 0,
	}, nil
}
