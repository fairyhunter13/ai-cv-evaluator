package usecase

import (
	"fmt"
	"strings"
	"time"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

type UploadService struct {
	Repo domain.UploadRepository
}

func NewUploadService(r domain.UploadRepository) UploadService { return UploadService{Repo: r} }

func (s UploadService) Ingest(ctx domain.Context, cvText, projText, cvName, projName string) (string, string, error) {
	cvText = sanitize(cvText)
	projText = sanitize(projText)
	if cvText == "" || projText == "" {
		return "", "", fmt.Errorf("%w: empty extracted text", domain.ErrInvalidArgument)
	}
	cvID, err := s.Repo.Create(ctx, domain.Upload{Type: domain.UploadTypeCV, Text: cvText, Filename: cvName, MIME: mimeFromName(cvName), Size: int64(len(cvText)), CreatedAt: time.Now().UTC()})
	if err != nil { return "", "", err }
	prjID, err := s.Repo.Create(ctx, domain.Upload{Type: domain.UploadTypeProject, Text: projText, Filename: projName, MIME: mimeFromName(projName), Size: int64(len(projText)), CreatedAt: time.Now().UTC()})
	if err != nil { return "", "", err }
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
