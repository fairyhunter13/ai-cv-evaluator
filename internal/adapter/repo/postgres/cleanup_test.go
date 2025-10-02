package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres/mocks"
)

func createMockTx(t *testing.T, commitErr error, rowErr error) *mocks.MockTx {
	mockTx := mocks.NewMockTx(t)
	mockTx.EXPECT().Commit(mock.Anything).Return(commitErr).Maybe()
	mockTx.EXPECT().Rollback(mock.Anything).Return(nil).Maybe()

	if rowErr != nil {
		mockRow := &mocks.MockRow{}
		mockRow.EXPECT().Scan(mock.Anything).Return(rowErr).Maybe()
		mockTx.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Maybe()
	} else {
		mockRow := &mocks.MockRow{}
		mockRow.EXPECT().Scan(mock.Anything).Run(func(dest ...any) {
			*(dest[0].(*int64)) = 1
		}).Return(nil).Maybe()
		mockTx.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Maybe()
	}

	return mockTx
}

func createMockBeginner(t *testing.T, beginErr error, tx *mocks.MockTx) *mocks.MockBeginner {
	mockBeginner := mocks.NewMockBeginner(t)
	if beginErr != nil {
		mockBeginner.EXPECT().Begin(mock.Anything).Return(nil, beginErr).Maybe()
	} else {
		mockBeginner.EXPECT().Begin(mock.Anything).Return(tx, nil).Maybe()
	}
	return mockBeginner
}

func TestCleanupService_CleanupOldData_OK(t *testing.T) {
	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}

func TestCleanupService_BeginError(t *testing.T) {
	b := createMockBeginner(t, errors.New("begin"), nil)
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCleanupService_CommitError(t *testing.T) {
	t.Helper()
	tx := createMockTx(t, errors.New("commit"), nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil {
		t.Fatalf("expected commit error")
	}
}

func TestCleanupService_RunPeriodic_ImmediateCancel(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 1)
	// Ensure it returns when context is canceled quickly
	go svc.RunPeriodic(ctx, 0)
}

func TestNewCleanupService_ZeroRetentionDays(t *testing.T) {
	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 0)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewCleanupService_NegativeRetentionDays(t *testing.T) {
	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, -1)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewCleanupService_LargeRetentionDays(t *testing.T) {
	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 365)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCleanupService_RunPeriodic_WithInterval(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tx := createMockTx(t, nil, nil)
	b := createMockBeginner(t, nil, tx)
	svc := postgres.NewCleanupService(b, 1)
	// Run with a short interval and timeout
	svc.RunPeriodic(ctx, 50*time.Millisecond)
}

func TestCleanupService_RunPeriodic_WithError(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use a beginner that will cause errors
	b := createMockBeginner(t, errors.New("begin error"), nil)
	svc := postgres.NewCleanupService(b, 1)
	// Run with a short interval and timeout
	svc.RunPeriodic(ctx, 50*time.Millisecond)
}
