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

func TestJobRepo_Create(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	job := domain.Job{
		ID:        "job-1",
		Status:    domain.JobQueued,
		CVID:      "cv-1",
		ProjectID: "proj-1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	// Test successful creation
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	id, err := repo.Create(ctx, job)
	require.NoError(t, err)
	assert.Equal(t, "job-1", id)

	// Test database error
	pool.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, assert.AnError).Once()
	_, err = repo.Create(ctx, job)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.create")
}

func TestJobRepo_UpdateStatus(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Create a mock transaction
	mockTx := mocks.NewMockTx(t)

	// Test successful update
	pool.EXPECT().BeginTx(mock.Anything, mock.Anything).Return(mockTx, nil).Once()
	mockTx.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockTx.EXPECT().Commit(mock.Anything).Return(nil).Once()
	mockTx.EXPECT().Rollback(mock.Anything).Return(nil).Once() // Rollback is called in defer after commit
	err := repo.UpdateStatus(ctx, "job-1", domain.JobCompleted, nil)
	require.NoError(t, err)

	// Test with error message
	errorMsg := "test error"
	pool.EXPECT().BeginTx(mock.Anything, mock.Anything).Return(mockTx, nil).Once()
	mockTx.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, nil).Once()
	mockTx.EXPECT().Commit(mock.Anything).Return(nil).Once()
	mockTx.EXPECT().Rollback(mock.Anything).Return(nil).Once() // Rollback is called in defer after commit
	err = repo.UpdateStatus(ctx, "job-1", domain.JobFailed, &errorMsg)
	require.NoError(t, err)

	// Test database error
	pool.EXPECT().BeginTx(mock.Anything, mock.Anything).Return(mockTx, nil).Once()
	mockTx.EXPECT().Exec(mock.Anything, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, assert.AnError).Once()
	mockTx.EXPECT().Rollback(mock.Anything).Return(nil).Once() // Rollback on error
	err = repo.UpdateStatus(ctx, "job-1", domain.JobCompleted, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.update_status")
}

func TestJobRepo_Get(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful get
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		*(dest[7].(**string)) = nil
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.MatchedBy(func(interface{}) bool { return true }), mock.Anything, mock.Anything).Return(mockRow).Once()

	job, err := repo.Get(ctx, "job-1")
	require.NoError(t, err)
	assert.Equal(t, "job-1", job.ID)
	assert.Equal(t, domain.JobCompleted, job.Status)

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.MatchedBy(func(interface{}) bool { return true }), mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err = repo.Get(ctx, "job-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.get")
}

func TestJobRepo_FindByIdempotencyKey(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful find
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		*(dest[7].(**string)) = nil
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()

	job, err := repo.FindByIdempotencyKey(ctx, "key-1")
	require.NoError(t, err)
	assert.Equal(t, "job-1", job.ID)

	// Test not found
	mockRowNotFound := mocks.NewMockRow(t)
	mockRowNotFound.On("Scan", mock.Anything).Return(domain.ErrNotFound).Once()
	pool.EXPECT().QueryRow(mock.MatchedBy(func(interface{}) bool { return true }), mock.Anything, mock.Anything).Return(mockRowNotFound).Once()
	_, err = repo.FindByIdempotencyKey(ctx, "key-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.find_idem")
}

func TestJobRepo_Count_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
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

func TestJobRepo_Count_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.Count(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.count")
}

func TestJobRepo_CountByStatus_Success(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful count by status
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*int64)) = int64(3)
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()

	count, err := repo.CountByStatus(ctx, domain.JobCompleted)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestJobRepo_CountByStatus_Error(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err := repo.CountByStatus(ctx, domain.JobCompleted)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.count_by_status")
}

func TestJobRepo_List(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful list with multiple jobs
	mockRows := mocks.NewMockRows(t)
	jobCount := 0
	mockRows.On("Next").Return(func() bool {
		jobCount++
		return jobCount <= 2 // Return 2 jobs
	}).Times(3) // Called 3 times: twice for jobs, once to return false

	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		jobID := "job-" + string(rune('0'+jobCount))
		*(dest[0].(*string)) = jobID
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-" + string(rune('0'+jobCount))
		*(dest[6].(*string)) = "proj-" + string(rune('0'+jobCount))
		*(dest[7].(**string)) = nil
	}).Return(nil).Times(2)

	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(nil).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
	assert.Equal(t, "job-1", jobs[0].ID)
	assert.Equal(t, "job-2", jobs[1].ID)
}

func TestJobRepo_List_WithIdempotencyKey(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test list with idempotency key
	mockRows := mocks.NewMockRows(t)
	jobCount := 0
	mockRows.On("Next").Return(func() bool {
		jobCount++
		return jobCount <= 1 // Return 1 job
	}).Times(2) // Called twice: once for job, once to return false

	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		idemKey := "idem-key-1"
		*(dest[7].(**string)) = &idemKey
	}).Return(nil).Once()

	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(nil).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "job-1", jobs[0].ID)
	assert.NotNil(t, jobs[0].IdemKey)
	assert.Equal(t, "idem-key-1", *jobs[0].IdemKey)
}

func TestJobRepo_List_WithError(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test list with error in job
	mockRows := mocks.NewMockRows(t)
	jobCount := 0
	mockRows.On("Next").Return(func() bool {
		jobCount++
		return jobCount <= 1 // Return 1 job
	}).Times(2) // Called twice: once for job, once to return false

	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobFailed
		*(dest[2].(*string)) = "test error"
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		*(dest[7].(**string)) = nil
	}).Return(nil).Once()

	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(nil).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "job-1", jobs[0].ID)
	assert.Equal(t, domain.JobFailed, jobs[0].Status)
	assert.Equal(t, "test error", jobs[0].Error)
}

func TestJobRepo_List_QueryError(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test query error
	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()
	jobs, err := repo.List(ctx, 0, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.list")
	assert.Nil(t, jobs)
}

func TestJobRepo_List_ScanError(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test scan error
	mockRows := mocks.NewMockRows(t)
	jobCount := 0
	mockRows.On("Next").Return(func() bool {
		jobCount++
		return jobCount <= 1 // Return 1 job
	}).Once() // Called once for the job

	mockRows.On("Scan", mock.Anything).Return(assert.AnError).Once()
	mockRows.On("Close").Return().Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.list_scan")
	assert.Nil(t, jobs)
}

func TestJobRepo_List_RowsError(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test rows error
	mockRows := mocks.NewMockRows(t)
	jobCount := 0
	mockRows.On("Next").Return(func() bool {
		jobCount++
		return jobCount <= 1 // Return 1 job
	}).Times(2) // Called twice: once for job, once to return false

	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now().UTC()
		*(dest[4].(*time.Time)) = time.Now().UTC()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		*(dest[7].(**string)) = nil
	}).Return(nil).Once()

	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(assert.AnError).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.list_rows")
	assert.Nil(t, jobs)
}

func TestJobRepo_List_EmptyResult(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test empty result
	mockRows := mocks.NewMockRows(t)
	mockRows.On("Next").Return(false).Once()
	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(nil).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.List(ctx, 0, 10)
	require.NoError(t, err)
	assert.Len(t, jobs, 0)
}

func TestJobRepo_GetAverageProcessingTime(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful get average processing time
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		avg := 1.5
		*(dest[0].(**float64)) = &avg
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()

	avg, err := repo.GetAverageProcessingTime(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1.5, avg)

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.MatchedBy(func(interface{}) bool { return true }), mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err = repo.GetAverageProcessingTime(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.avg_processing_time")
}

func TestJobRepo_ListWithFilters(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful list with no filters
	mockRows := mocks.NewMockRows(t)
	mockRows.On("Next").Return(true).Once()
	mockRows.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		*(dest[0].(*string)) = "job-1"
		*(dest[1].(*domain.JobStatus)) = domain.JobCompleted
		*(dest[2].(*string)) = ""
		*(dest[3].(*time.Time)) = time.Now()
		*(dest[4].(*time.Time)) = time.Now()
		*(dest[5].(*string)) = "cv-1"
		*(dest[6].(*string)) = "proj-1"
		*(dest[7].(**string)) = nil
	}).Return(nil).Once()
	mockRows.On("Next").Return(false).Once()
	mockRows.On("Close").Return().Once()
	mockRows.On("Err").Return(nil).Once()

	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil).Once()

	jobs, err := repo.ListWithFilters(ctx, 0, 10, "", "")
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "job-1", jobs[0].ID)

	// Test database error
	pool.EXPECT().Query(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()
	_, err = repo.ListWithFilters(ctx, 0, 10, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.list_with_filters")
}

func TestJobRepo_CountWithFilters(t *testing.T) {
	pool := postgres.NewMockPgxPool(t)
	repo := postgres.NewJobRepo(pool)
	ctx := context.Background()

	// Test successful count with no filters
	mockRow := mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		count := int64(5)
		*(dest[0].(*int64)) = count
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()
	count, err := repo.CountWithFilters(ctx, "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// Test successful count with search filter
	mockRow = mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		count := int64(2)
		*(dest[0].(*int64)) = count
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()
	count, err = repo.CountWithFilters(ctx, "job-1", "")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Test successful count with status filter
	mockRow = mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		count := int64(3)
		*(dest[0].(*int64)) = count
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()
	count, err = repo.CountWithFilters(ctx, "", "completed")
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Test successful count with both filters
	mockRow = mocks.NewMockRow(t)
	mockRow.On("Scan", mock.Anything).Run(func(args mock.Arguments) {
		dest := args[0].([]any)
		count := int64(1)
		*(dest[0].(*int64)) = count
	}).Return(nil).Once()

	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRow).Once()
	count, err = repo.CountWithFilters(ctx, "job-1", "completed")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test database error
	mockRowErr := mocks.NewMockRow(t)
	mockRowErr.On("Scan", mock.Anything).Return(assert.AnError).Once()
	pool.EXPECT().QueryRow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(mockRowErr).Once()
	_, err = repo.CountWithFilters(ctx, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "op=job.count_with_filters")
}
