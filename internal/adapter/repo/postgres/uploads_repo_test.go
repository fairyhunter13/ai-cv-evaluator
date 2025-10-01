package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestUploadRepo_Create_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	upload := domain.Upload{
		ID:        "upload-1",
		Type:      domain.UploadTypeCV,
		Text:      "test content",
		Filename:  "test.txt",
		MIME:      "text/plain",
		Size:      100,
		CreatedAt: time.Now().UTC(),
	}

	// Test successful creation
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	id, err := repo.Create(ctx, upload)
	require.NoError(t, err)
	assert.Equal(t, "upload-1", id)
}

func TestUploadRepo_Create_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	upload := domain.Upload{
		ID:        "upload-1",
		Type:      domain.UploadTypeCV,
		Text:      "test content",
		Filename:  "test.txt",
		MIME:      "text/plain",
		Size:      100,
		CreatedAt: time.Now().UTC(),
	}

	// Test database error
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, assert.AnError).Once()
	_, err := repo.Create(ctx, upload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=upload.create")
}

func TestUploadRepo_Get_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test successful get
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "upload-1"
		*(dest[1].(*string)) = domain.UploadTypeCV
		*(dest[2].(*string)) = "test content"
		*(dest[3].(*string)) = "test.txt"
		*(dest[4].(*string)) = "text/plain"
		*(dest[5].(*int64)) = int64(100)
		*(dest[6].(*time.Time)) = time.Now().UTC()
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()

	upload, err := repo.Get(ctx, "upload-1")
	require.NoError(t, err)
	assert.Equal(t, "upload-1", upload.ID)
	assert.Equal(t, domain.UploadTypeCV, upload.Type)
}

func TestUploadRepo_Get_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.Get(ctx, "upload-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=upload.get")
}

func TestUploadRepo_Count_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test successful count
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*int64)) = int64(5)
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything).Return(mockRow).Once()

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestUploadRepo_Count_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.Count(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=upload.count")
}

func TestUploadRepo_CountByType_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test successful count by type
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*int64)) = int64(3)
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()

	count, err := repo.CountByType(ctx, domain.UploadTypeCV)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestUploadRepo_CountByType_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewUploadRepo(pool)
	ctx := context.Background()

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.CountByType(ctx, domain.UploadTypeCV)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=upload.count_by_type")
}
