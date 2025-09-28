// Package app wires application components and startup helpers.
package app

import (
	"context"
	"log/slog"

	qdrantcli "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/vector/qdrant"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/ragseed"
)

// EnsureDefaultCollections ensures collections exist and seeds them using ragseed.
func EnsureDefaultCollections(ctx context.Context, qcli *qdrantcli.Client, aicl domain.AIClient) {
	if qcli == nil { return }
	if err := qcli.EnsureCollection(ctx, "job_description", 1536, "Cosine"); err != nil {
		slog.Warn("qdrant ensure job_description failed", slog.Any("error", err))
	}
	if err := qcli.EnsureCollection(ctx, "scoring_rubric", 1536, "Cosine"); err != nil {
		slog.Warn("qdrant ensure scoring_rubric failed", slog.Any("error", err))
	}
	if aicl != nil {
		_ = ragseed.SeedDefault(ctx, qcli, aicl)
	}
}
