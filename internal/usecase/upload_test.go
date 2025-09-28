package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"fmt"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

type stubUploadRepo2 struct{ created []domain.Upload; idSeq int; err error }
func (r *stubUploadRepo2) Create(ctx domain.Context, u domain.Upload) (string, error) {
	r.created = append(r.created, u)
	if r.err != nil { return "", r.err }
	r.idSeq++
	return time.Now().Format("150405") + "-" + fmt.Sprintf("%d", r.idSeq), nil
}
func (r *stubUploadRepo2) Get(ctx domain.Context, id string) (domain.Upload, error) { return domain.Upload{ID:id}, nil }

func TestUpload_Ingest_Success(t *testing.T) {
	t.Parallel()
	repo := &stubUploadRepo2{}
	svc := usecase.NewUploadService(repo)
	cvID, prID, err := svc.Ingest(context.Background(), "hello cv", "hello pr", "cv.docx", "pr.pdf")
	require.NoError(t, err)
	assert.NotEmpty(t, cvID)
	assert.NotEmpty(t, prID)
	require.Len(t, repo.created, 2)
	assert.Equal(t, domain.UploadTypeCV, repo.created[0].Type)
	assert.Equal(t, domain.UploadTypeProject, repo.created[1].Type)
}

func TestUpload_Ingest_EmptyRejected(t *testing.T) {
	t.Parallel()
	repo := &stubUploadRepo2{}
	svc := usecase.NewUploadService(repo)
	_, _, err := svc.Ingest(context.Background(), "", "hello pr", "cv.txt", "pr.txt")
	require.Error(t, err)
}
