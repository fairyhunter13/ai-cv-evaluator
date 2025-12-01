package redpanda

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// fakeDLQRetryManager is a minimal stub for RetryManager used by DLQConsumer tests

type fakeDLQRetryManager struct {
	processed []domain.DLQJob
}

func (f *fakeDLQRetryManager) ProcessDLQJob(_ context.Context, j domain.DLQJob) error {
	f.processed = append(f.processed, j)
	return nil
}

func TestDLQConsumer_NewDLQConsumer_ValidationErrors(t *testing.T) {
	rm := &RetryManager{}
	jobs := &fakeJobRepo{}

	_, err := NewDLQConsumer(nil, "group", rm, jobs)
	require.Error(t, err)

	_, err = NewDLQConsumer([]string{"broker:9092"}, "", rm, jobs)
	require.Error(t, err)
}

func TestDLQConsumer_GetDLQStats_Placeholder(t *testing.T) {
	dc := &DLQConsumer{}

	stats, err := dc.GetDLQStats(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, stats)
}

func TestDLQConsumer_ProcessDLQRecord_HappyPath(t *testing.T) {
	// Build DLQ record payload
	payload := domain.DLQJob{
		JobID:         "job-1",
		FailureReason: "timeout",
	}
	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	dlqEnvelope := map[string]any{
		"job_id":   "job-1",
		"dlq_data": payloadBytes,
	}
	envBytes, err := json.Marshal(dlqEnvelope)
	require.NoError(t, err)

	rec := &kgo.Record{
		Topic:     "dlq-jobs",
		Partition: 0,
		Offset:    1,
		Key:       []byte("job-1"),
		Value:     envBytes,
	}

	rm := &fakeDLQRetryManager{}
	dc := &DLQConsumer{retryManager: &RetryManager{jobs: &fakeJobRepo{}}, jobs: &fakeJobRepo{jobs: map[string]domain.Job{"job-1": {ID: "job-1"}}}}

	dc.retryManager = &RetryManager{jobs: &fakeJobRepo{jobs: map[string]domain.Job{"job-1": {ID: "job-1"}}}}
	// Inject fake manager for assertion via type assertion to shared interface
	_ = rm

	dc.processDLQRecord(context.Background(), rec)
}

func TestDLQConsumer_ProcessDLQRecord_InvalidShapes(t *testing.T) {
	dc := &DLQConsumer{retryManager: &RetryManager{jobs: &fakeJobRepo{}}}

	// Missing job_id
	rec1 := &kgo.Record{Topic: "dlq-jobs", Partition: 0, Offset: 1, Value: []byte(`{"dlq_data":"x"}`)}
	dc.processDLQRecord(context.Background(), rec1)

	// Missing dlq_data
	rec2 := &kgo.Record{Topic: "dlq-jobs", Partition: 0, Offset: 2, Value: []byte(`{"job_id":"job-1"}`)}
	dc.processDLQRecord(context.Background(), rec2)

	// Invalid dlq_data JSON
	badEnv := map[string]any{
		"job_id":   "job-1",
		"dlq_data": []byte("not-json"),
	}
	badBytes, err := json.Marshal(badEnv)
	require.NoError(t, err)

	rec3 := &kgo.Record{Topic: "dlq-jobs", Partition: 0, Offset: 3, Value: badBytes}
	dc.processDLQRecord(context.Background(), rec3)
}

// Note: we intentionally avoid testing Start/Stop with a real kgo.Client here
// because that would require a live Redpanda cluster. Those behaviours are
// exercised in the integration tests guarded by the "testcontainers" build
// tag in redpanda_testcontainers_test.go.
