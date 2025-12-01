// Package postgres provides PostgreSQL database adapters.
//
// It implements repository interfaces for data persistence.
// The package provides type-safe database operations with
// connection pooling and transaction support.
package postgres

import (
	"fmt"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// ResultRepo persists and loads evaluation results from PostgreSQL.
type ResultRepo struct{ Pool PgxPool }

// NewResultRepo constructs a ResultRepo with the given pool.
func NewResultRepo(p PgxPool) *ResultRepo { return &ResultRepo{Pool: p} }

// Upsert inserts or updates a result by job_id.
func (r *ResultRepo) Upsert(ctx domain.Context, res domain.Result) error {
	tracer := otel.Tracer("repo.results")
	ctx, span := tracer.Start(ctx, "results.Upsert")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPSERT"),
		attribute.String("db.sql.table", "results"),
	)
	q := `INSERT INTO results (job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7)
	ON CONFLICT (job_id)
	DO UPDATE SET cv_match_rate=EXCLUDED.cv_match_rate, cv_feedback=EXCLUDED.cv_feedback, project_score=EXCLUDED.project_score, project_feedback=EXCLUDED.project_feedback, overall_summary=EXCLUDED.overall_summary`
	_, err := r.Pool.Exec(ctx, q, res.JobID, res.CVMatchRate, res.CVFeedback, res.ProjectScore, res.ProjectFeedback, res.OverallSummary, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("op=result.upsert: %w", err)
	}
	return nil
}

// GetByJobID loads a result by its job_id.
func (r *ResultRepo) GetByJobID(ctx domain.Context, jobID string) (domain.Result, error) {
	tracer := otel.Tracer("repo.results")
	ctx, span := tracer.Start(ctx, "results.GetByJobID")
	defer span.End()
	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "results"),
	)
	q := `SELECT job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at FROM results WHERE job_id=$1`
	row := r.Pool.QueryRow(ctx, q, jobID)
	var res domain.Result
	if err := row.Scan(&res.JobID, &res.CVMatchRate, &res.CVFeedback, &res.ProjectScore, &res.ProjectFeedback, &res.OverallSummary, &res.CreatedAt); err != nil {
		return domain.Result{}, fmt.Errorf("op=result.get: %w", err)
	}
	return res, nil
}
