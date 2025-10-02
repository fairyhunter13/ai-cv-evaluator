package usecase_test

import (
	"context"
	"testing"

	"fmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestUpload_Ingest_Success(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)

	// Set up expectations for two Create calls
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u domain.Upload) bool {
		return u.Type == domain.UploadTypeCV
	})).Return("cv-123", nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u domain.Upload) bool {
		return u.Type == domain.UploadTypeProject
	})).Return("pr-456", nil).Once()

	cvID, prID, err := svc.Ingest(context.Background(), "hello cv", "hello pr", "cv.docx", "pr.pdf")
	require.NoError(t, err)
	assert.NotEmpty(t, cvID)
	assert.NotEmpty(t, prID)
}

func TestUpload_Ingest_EmptyRejected(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)
	_, _, err := svc.Ingest(context.Background(), "", "hello pr", "cv.txt", "pr.txt")
	require.Error(t, err)
}

func TestUpload_Ingest_EmptyProjectRejected(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)
	_, _, err := svc.Ingest(context.Background(), "hello cv", "", "cv.txt", "pr.txt")
	require.Error(t, err)
}

func TestUpload_Ingest_BothEmptyRejected(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)
	_, _, err := svc.Ingest(context.Background(), "", "", "cv.txt", "pr.txt")
	require.Error(t, err)
}

func TestUpload_Ingest_RepositoryError(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)

	// Set up expectation for first Create call to fail
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u domain.Upload) bool {
		return u.Type == domain.UploadTypeCV
	})).Return("", fmt.Errorf("database error")).Once()

	_, _, err := svc.Ingest(context.Background(), "hello cv", "hello pr", "cv.txt", "pr.txt")
	require.Error(t, err)
}

func TestUpload_Ingest_WithDifferentFileTypes(t *testing.T) {
	t.Parallel()
	repo := mocks.NewMockUploadRepository(t)
	svc := usecase.NewUploadService(repo)

	// Set up expectations for two Create calls
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u domain.Upload) bool {
		return u.Type == domain.UploadTypeCV
	})).Return("cv-123", nil).Once()
	repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(u domain.Upload) bool {
		return u.Type == domain.UploadTypeProject
	})).Return("pr-456", nil).Once()

	cvID, prID, err := svc.Ingest(context.Background(), "hello cv", "hello pr", "cv.pdf", "pr.docx")
	require.NoError(t, err)
	assert.NotEmpty(t, cvID)
	assert.NotEmpty(t, prID)
}
