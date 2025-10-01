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
