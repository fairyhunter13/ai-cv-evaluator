package postgres

import (
	"fmt"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
)

type ResultRepo struct { Pool PgxPool }

func NewResultRepo(p PgxPool) *ResultRepo { return &ResultRepo{Pool: p} }

func (r *ResultRepo) Upsert(ctx domain.Context, res domain.Result) error {
	tracer := otel.Tracer("repo.results")
	ctx, span := tracer.Start(ctx, "results.Upsert")
	defer span.End()
	q := `INSERT INTO results (job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7)
	ON CONFLICT (job_id)
	DO UPDATE SET cv_match_rate=EXCLUDED.cv_match_rate, cv_feedback=EXCLUDED.cv_feedback, project_score=EXCLUDED.project_score, project_feedback=EXCLUDED.project_feedback, overall_summary=EXCLUDED.overall_summary`
	_, err := r.Pool.Exec(ctx, q, res.JobID, res.CVMatchRate, res.CVFeedback, res.ProjectScore, res.ProjectFeedback, res.OverallSummary, time.Now().UTC())
	if err != nil { return fmt.Errorf("op=result.upsert: %w", err) }
	return nil
}

func (r *ResultRepo) GetByJobID(ctx domain.Context, jobID string) (domain.Result, error) {
	tracer := otel.Tracer("repo.results")
	ctx, span := tracer.Start(ctx, "results.GetByJobID")
	defer span.End()
	q := `SELECT job_id, cv_match_rate, cv_feedback, project_score, project_feedback, overall_summary, created_at FROM results WHERE job_id=$1`
	row := r.Pool.QueryRow(ctx, q, jobID)
	var res domain.Result
	if err := row.Scan(&res.JobID, &res.CVMatchRate, &res.CVFeedback, &res.ProjectScore, &res.ProjectFeedback, &res.OverallSummary, &res.CreatedAt); err != nil {
		return domain.Result{}, fmt.Errorf("op=result.get: %w", err)
	}
	return res, nil
}
