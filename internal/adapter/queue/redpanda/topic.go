// Package redpanda provides Redpanda/Kafka queue integration.
//
// It handles message publishing and consumption for job processing.
// The package provides reliable message delivery with exactly-once
// semantics and supports horizontal scaling of workers.
package redpanda

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// createTopicIfNotExists creates a topic if it doesn't exist using the Kafka AdminClient API.
// It handles the "topic already exists" error gracefully and returns nil in that case.
// This function follows exactly-once semantics by ensuring the topic is ready before any
// producer or consumer operations.
func createTopicIfNotExists(ctx context.Context, client *kgo.Client, topic string, partitions int32, replicationFactor int16) error {
	// Validate input parameters
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	if partitions <= 0 {
		return fmt.Errorf("partitions must be greater than 0")
	}
	if replicationFactor <= 0 {
		return fmt.Errorf("replication factor must be greater than 0")
	}

	slog.Info("ensuring topic exists",
		slog.String("topic", topic),
		slog.Int("partitions", int(partitions)),
		slog.Int("replication_factor", int(replicationFactor)))

	// Create CreateTopicsRequest
	req := kmsg.NewCreateTopicsRequest()
	req.TimeoutMillis = 30000 // 30 seconds timeout

	topicReq := kmsg.NewCreateTopicsRequestTopic()
	topicReq.Topic = topic
	topicReq.NumPartitions = partitions
	topicReq.ReplicationFactor = replicationFactor

	req.Topics = append(req.Topics, topicReq)

	// Send request
	resp, err := client.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	// Check response
	createTopicsResp, ok := resp.(*kmsg.CreateTopicsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	for _, topicResp := range createTopicsResp.Topics {
		if topicResp.ErrorCode != 0 {
			// Check if topic already exists (error code 36 = TOPIC_ALREADY_EXISTS)
			// Reference: https://kafka.apache.org/protocol#protocol_error_codes
			if topicResp.ErrorCode == 36 {
				slog.Info("topic already exists", slog.String("topic", topicResp.Topic))
				return nil
			}
			errorMsg := ""
			if topicResp.ErrorMessage != nil {
				errorMsg = *topicResp.ErrorMessage
			}
			return fmt.Errorf("create topic error: %s (code %d)", errorMsg, topicResp.ErrorCode)
		}
		slog.Info("topic created successfully",
			slog.String("topic", topicResp.Topic),
			slog.Int("partitions", int(partitions)),
			slog.Int("replication_factor", int(replicationFactor)))
	}

	return nil
}

// createOptimizedTopicForParallelProcessing creates a topic optimized for parallel processing.
// This function creates topics with multiple partitions and optimized settings for E2E testing.
func createOptimizedTopicForParallelProcessing(ctx context.Context, client *kgo.Client, topic string, partitions int32, replicationFactor int16) error {
	// Validate input parameters
	if topic == "" {
		return fmt.Errorf("topic name cannot be empty")
	}
	if partitions <= 0 {
		return fmt.Errorf("partitions must be greater than 0")
	}
	if replicationFactor <= 0 {
		return fmt.Errorf("replication factor must be greater than 0")
	}

	slog.Info("creating optimized topic for parallel processing",
		slog.String("topic", topic),
		slog.Int("partitions", int(partitions)),
		slog.Int("replication_factor", int(replicationFactor)))

	// Create CreateTopicsRequest with optimized settings
	req := kmsg.NewCreateTopicsRequest()
	req.TimeoutMillis = 30000 // 30 seconds timeout

	topicReq := kmsg.NewCreateTopicsRequestTopic()
	topicReq.Topic = topic
	topicReq.NumPartitions = partitions
	topicReq.ReplicationFactor = replicationFactor

	// Optimized topic configuration for parallel processing
	topicReq.Configs = []kmsg.CreateTopicsRequestTopicConfig{
		{Name: "cleanup.policy", Value: stringPtr("delete")},
		{Name: "retention.ms", Value: stringPtr("604800000")}, // 7 days
		{Name: "segment.ms", Value: stringPtr("3600000")},     // 1 hour
		{Name: "compression.type", Value: stringPtr("snappy")},
		{Name: "min.insync.replicas", Value: stringPtr("1")},
		{Name: "unclean.leader.election.enable", Value: stringPtr("false")},
		{Name: "min.compaction.lag.ms", Value: stringPtr("0")},
		{Name: "max.compaction.lag.ms", Value: stringPtr("9223372036854775807")},
		{Name: "message.timestamp.type", Value: stringPtr("CreateTime")},
		{Name: "message.timestamp.difference.max.ms", Value: stringPtr("9223372036854775807")},
		{Name: "segment.index.bytes", Value: stringPtr("10485760")}, // 10MB
		{Name: "segment.jitter.ms", Value: stringPtr("0")},
		{Name: "preallocate", Value: stringPtr("false")},
		{Name: "min.cleanable.dirty.ratio", Value: stringPtr("0.5")},
		{Name: "delete.retention.ms", Value: stringPtr("86400000")}, // 1 day
		{Name: "file.delete.delay.ms", Value: stringPtr("60000")},   // 1 minute
		{Name: "flush.messages", Value: stringPtr("9223372036854775807")},
		{Name: "flush.ms", Value: stringPtr("9223372036854775807")},
		{Name: "index.interval.bytes", Value: stringPtr("4096")},
		{Name: "max.message.bytes", Value: stringPtr("1000012")},
		{Name: "message.format.version", Value: stringPtr("2.8-IV1")},
		{Name: "segment.bytes", Value: stringPtr("1073741824")}, // 1GB
		{Name: "retention.bytes", Value: stringPtr("-1")},
	}

	req.Topics = append(req.Topics, topicReq)

	// Send request
	resp, err := client.Request(ctx, &req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	// Check response
	createTopicsResp, ok := resp.(*kmsg.CreateTopicsResponse)
	if !ok {
		return fmt.Errorf("unexpected response type: %T", resp)
	}

	for _, topicResp := range createTopicsResp.Topics {
		if topicResp.ErrorCode != 0 {
			// Check if topic already exists (error code 36 = TOPIC_ALREADY_EXISTS)
			if topicResp.ErrorCode == 36 {
				slog.Info("optimized topic already exists", slog.String("topic", topicResp.Topic))
				return nil
			}
			errorMsg := ""
			if topicResp.ErrorMessage != nil {
				errorMsg = *topicResp.ErrorMessage
			}
			return fmt.Errorf("create optimized topic error: %s (code %d)", errorMsg, topicResp.ErrorCode)
		}
		slog.Info("optimized topic created successfully for parallel processing",
			slog.String("topic", topicResp.Topic),
			slog.Int("partitions", int(partitions)),
			slog.Int("replication_factor", int(replicationFactor)))
	}

	return nil
}

// stringPtr returns a pointer to a string value.
func stringPtr(s string) *string {
	return &s
}
