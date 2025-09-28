package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CleanupService handles data retention and cleanup
type CleanupService struct {
	Pool       *pgxpool.Pool
	RetentionDays int
}

// NewCleanupService creates a new cleanup service
func NewCleanupService(pool *pgxpool.Pool, retentionDays int) *CleanupService {
	if retentionDays <= 0 {
		retentionDays = 90 // default 90 days
	}
	return &CleanupService{Pool: pool, RetentionDays: retentionDays}
}

// CleanupOldData removes data older than retention period
func (s *CleanupService) CleanupOldData(ctx context.Context) error {
	cutoff := time.Now().AddDate(0, 0, -s.RetentionDays)
	
	// Start transaction for consistency
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("cleanup begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Delete old results (cascade from jobs)
	var deletedResults int64
	err = tx.QueryRow(ctx, `
		DELETE FROM results 
		WHERE job_id IN (
			SELECT id FROM jobs WHERE created_at < $1
		)
		RETURNING count(*)
	`, cutoff).Scan(&deletedResults)
	if err != nil {
		slog.Debug("no results to delete", slog.Any("error", err))
	}

	// Delete old jobs
	var deletedJobs int64
	err = tx.QueryRow(ctx, `
		DELETE FROM jobs 
		WHERE created_at < $1
		RETURNING count(*)
	`, cutoff).Scan(&deletedJobs)
	if err != nil {
		slog.Debug("no jobs to delete", slog.Any("error", err))
	}

	// Delete orphaned uploads (not referenced by any job)
	var deletedUploads int64
	err = tx.QueryRow(ctx, `
		DELETE FROM uploads 
		WHERE created_at < $1 
		AND id NOT IN (
			SELECT cv_id FROM jobs 
			UNION 
			SELECT project_id FROM jobs
		)
		RETURNING count(*)
	`, cutoff).Scan(&deletedUploads)
	if err != nil {
		slog.Debug("no uploads to delete", slog.Any("error", err))
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("cleanup commit: %w", err)
	}

	slog.Info("data cleanup completed",
		slog.Int64("deleted_jobs", deletedJobs),
		slog.Int64("deleted_results", deletedResults),
		slog.Int64("deleted_uploads", deletedUploads),
		slog.Time("cutoff", cutoff),
	)

	return nil
}

// RunPeriodic starts a periodic cleanup job
func (s *CleanupService) RunPeriodic(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 24 * time.Hour // daily by default
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial cleanup
	if err := s.CleanupOldData(ctx); err != nil {
		slog.Error("initial cleanup failed", slog.Any("error", err))
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("cleanup service stopping")
			return
		case <-ticker.C:
			if err := s.CleanupOldData(ctx); err != nil {
				slog.Error("periodic cleanup failed", slog.Any("error", err))
			}
		}
	}
}
