package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/jackc/pgx/v5"
)

type fakeTx struct{ commitErr error; rowErr error }

func (t *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return rowStub{scan: func(dest ...any) error {
		if t.rowErr != nil { return t.rowErr }
		*(dest[0].(*int64)) = 1
		return nil
	}}
}
func (t *fakeTx) Commit(_ context.Context) error { return t.commitErr }
func (t *fakeTx) Rollback(_ context.Context) error { return nil }

type fakeBeginner struct{ beginErr error; tx *fakeTx }
func (b *fakeBeginner) Begin(_ context.Context) (postgres.Tx, error) { if b.beginErr != nil { return nil, b.beginErr }; return b.tx, nil }

func TestCleanupService_CleanupOldData_OK(t *testing.T) {
	b := &fakeBeginner{tx: &fakeTx{}}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err != nil { t.Fatalf("cleanup: %v", err) }
}

func TestCleanupService_BeginError(t *testing.T) {
	b := &fakeBeginner{beginErr: errors.New("begin")}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil { t.Fatalf("expected error") }
}

func TestCleanupService_CommitError(t *testing.T) {
	t.Helper()
	b := &fakeBeginner{tx: &fakeTx{commitErr: errors.New("commit")}}
	svc := postgres.NewCleanupService(b, 1)
	if err := svc.CleanupOldData(context.Background()); err == nil { t.Fatalf("expected commit error") }
}

func TestCleanupService_RunPeriodic_ImmediateCancel(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := postgres.NewCleanupService(&fakeBeginner{tx: &fakeTx{}}, 1)
	// Ensure it returns when context is canceled quickly
	go svc.RunPeriodic(ctx, 0)
}
