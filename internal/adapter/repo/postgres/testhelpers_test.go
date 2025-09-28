package postgres_test

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// rowStub implements pgx.Row
type rowStub struct{ scan func(dest ...any) error }
func (r rowStub) Scan(dest ...any) error { return r.scan(dest...) }

// poolStub implements postgres.PgxPool for tests
// It stubs Exec and QueryRow behavior
// Define in a shared helper so multiple *_test.go files can reuse it without redefs

type poolStub struct{
	execErr error
	row    rowStub
}

func (p *poolStub) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, p.execErr
}

func (p *poolStub) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if p.row.scan == nil { return rowStub{scan: func(_ ...any) error { return errors.New("no row configured") }} }
	return p.row
}
