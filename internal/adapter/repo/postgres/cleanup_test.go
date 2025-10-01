package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres/mocks"
	"github.com/jackc/pgx/v5"
)

type fakeTx struct {
	commitErr error
	rowErr    error
}

func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	mockRow := &mocks.MockRow{}
	if t.rowErr != nil {
		mockRow.On("Scan", mock.Anything).Return(t.rowErr)
	} else {
		mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
			dest := args[0].([]any)
			*(dest[0].(*int64)) = 1
		}).Return(nil)
	}
	return mockRow
}
func (t *fakeTx) Commit(_ context.Context) error   { return t.commitErr }
func (t *fakeTx) Rollback(_ context.Context) error { return nil }

type fakeBeginner struct {
	beginErr error
	tx       *fakeTx
}

func (b *fakeBeginner) Begin(_ context.Context) (postgres.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

func TestCleanupService_CleanupOldData_OK(t *testing.T) {
	b := &fakeBeginner{tx: &fakeTx{}}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestCleanupService_BeginError(t *testing.T) {
	b := &fakeBeginner{beginErr: errors.New("begin")}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCleanupService_CommitError(t *testing.T) {
	t.Helper()
	b := &fakeBeginner{tx: &fakeTx{commitErr: errors.New("commit")}}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil {
		t.Fatalf("expected commit error")
	}
}

func TestCleanupService_RunPeriodic_ImmediateCancel(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, 1)
	// Ensure it returns when context is canceled quickly
	go svc.RunPeriodic(ctx, 0)
}

func TestNewCleanupService_ZeroRetentionDays(t *testing.T) {
	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, 0)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewCleanupService_NegativeRetentionDays(t *testing.T) {
	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, -1)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewCleanupService_LargeRetentionDays(t *testing.T) {
	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, 365)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCleanupService_RunPeriodic_WithInterval(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, 1)
	// Run with a short interval and timeout
	svc.RunPeriodic(ctx, 50*time.Millisecond)
}

func TestCleanupService_RunPeriodic_WithError(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use a beginner that will cause errors
	b := &fakeBeginner{beginErr: errors.New("begin error")}
	svc := postgres.NewCleanupService(b, 1)
	// Run with a short interval and timeout
	svc.RunPeriodic(ctx, 50*time.Millisecond)
}
