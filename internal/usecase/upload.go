package usecase

import (
	"fmt"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// UploadService ingests sanitized texts and persists them via the repository.
type UploadService struct {
	Repo domain.UploadRepository
}

// NewUploadService constructs an UploadService with the given repo.
func NewUploadService(r domain.UploadRepository) UploadService { return UploadService{Repo: r} }

// Ingest sanitizes input texts, validates non-empty content, and stores both
// the CV and project uploads, returning their generated ids.
func (s UploadService) Ingest(ctx domain.Context, cvText, projText, cvName, projName string) (string, string, error) {
	cvText = sanitize(cvText)
	projText = sanitize(projText)
	if cvText == "" || projText == "" {
		return "", "", fmt.Errorf("%w: empty extracted text", domain.ErrInvalidArgument)
	}
	cvID, err := s.Repo.Create(ctx, domain.Upload{Type: domain.UploadTypeCV, Text: cvText, Filename: cvName, MIME: mimeFromName(cvName), Size: int64(len(cvText)), CreatedAt: time.Now().UTC()})
	if err != nil {
		return "", "", err
	}
	prjID, err := s.Repo.Create(ctx, domain.Upload{Type: domain.UploadTypeProject, Text: projText, Filename: projName, MIME: mimeFromName(projName), Size: int64(len(projText)), CreatedAt: time.Now().UTC()})
	if err != nil {
		return "", "", err
	}
	return cvID, prjID, nil
}

func sanitize(s string) string { return strings.TrimSpace(s) }

func mimeFromName(n string) string {
	n = strings.ToLower(n)
	switch {
	case strings.HasSuffix(n, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(n, ".docx"):
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "text/plain"
	}
}

// Count returns the total number of uploads.
func (s UploadService) Count(ctx domain.Context) (int64, error) {
	return s.Repo.Count(ctx)
}

// CountByType returns the number of uploads by type.
func (s UploadService) CountByType(ctx domain.Context, uploadType string) (int64, error) {
	return s.Repo.CountByType(ctx, uploadType)
}
