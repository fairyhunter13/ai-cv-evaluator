package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type StuckJobSweeper struct {
	jobs             domain.JobRepository
	maxProcessingAge time.Duration
	interval         time.Duration
}

func NewStuckJobSweeper(jobs domain.JobRepository, maxProcessingAge, interval time.Duration) *StuckJobSweeper {
	if jobs == nil {
		return nil
	}
	if maxProcessingAge <= 0 {
		maxProcessingAge = 3 * time.Minute
	}
	if interval <= 0 {
		interval = time.Minute
	}
	return &StuckJobSweeper{
		jobs:             jobs,
		maxProcessingAge: maxProcessingAge,
		interval:         interval,
	}
}

func (s *StuckJobSweeper) Run(ctx context.Context) {
	if s == nil || s.jobs == nil {
		return
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.sweepOnce(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stuck job sweeper stopping")
			return
		case <-ticker.C:
			s.sweepOnce(ctx)
		}
	}
}

func (s *StuckJobSweeper) sweepOnce(ctx context.Context) {
	tracer := otel.Tracer("jobs.sweeper")
	ctx, span := tracer.Start(ctx, "StuckJobSweeper.sweepOnce")
	defer span.End()

	cutoff := time.Now().Add(-s.maxProcessingAge)
	const pageSize = 100
	span.SetAttributes(
		attribute.Int("jobs.page_size", pageSize),
		attribute.Float64("jobs.max_processing_age_seconds", s.maxProcessingAge.Seconds()),
	)

	totalChecked := 0
	totalMarkedFailed := 0

	for offset := 0; ; offset += pageSize {
		pageCtx, pageSpan := tracer.Start(ctx, "StuckJobSweeper.sweepPage")
		pageSpan.SetAttributes(attribute.Int("jobs.offset", offset))

		jobs, err := s.jobs.ListWithFilters(pageCtx, offset, pageSize, "", string(domain.JobProcessing))
		if err != nil {
			pageSpan.RecordError(err)
			pageSpan.End()
			slog.Error("stuck job sweep failed to list jobs", slog.Any("error", err))
			return
		}
		totalChecked += len(jobs)
		if len(jobs) == 0 {
			pageSpan.End()
			break
		}

		for _, j := range jobs {
			if j.UpdatedAt.Before(cutoff) {
				jobCtx, jobSpan := tracer.Start(pageCtx, "StuckJobSweeper.markFailed")
				jobSpan.SetAttributes(
					attribute.String("job.id", j.ID),
					attribute.String("job.status", string(j.Status)),
				)
				msg := fmt.Sprintf("job processing exceeded maximum age %v; marking as failed by sweeper", s.maxProcessingAge)
				if err := s.jobs.UpdateStatus(jobCtx, j.ID, domain.JobFailed, &msg); err != nil {
					jobSpan.RecordError(err)
					slog.Error("stuck job sweep failed to update job status", slog.String("job_id", j.ID), slog.Any("error", err))
				} else {
					totalMarkedFailed++
				}
				jobSpan.End()
			}
		}

		pageSpan.End()

		if len(jobs) < pageSize {
			break
		}
	}

	span.SetAttributes(
		attribute.Int("jobs.total_checked", totalChecked),
		attribute.Int("jobs.total_marked_failed", totalMarkedFailed),
	)
}
