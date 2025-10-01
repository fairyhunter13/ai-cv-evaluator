package httpserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/httpserver"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestAdminStatsHandler_Unauthorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/stats", nil)
	w := httptest.NewRecorder()

	adminServer.AdminStatsHandler()(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminStatsHandler_Authorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}

	// Create a proper server with repositories
	upRepo := createMockUploadRepoForAdmin(t)
	jobRepo := createMockJobRepoForAdmin(t)
	resRepo := createMockResultRepoForAdmin(t)
	server := httpserver.NewServer(cfg, usecase.NewUploadService(upRepo), usecase.NewEvaluateService(jobRepo, nil, upRepo), usecase.NewResultService(jobRepo, resRepo), nil, nil, nil, nil)

	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	// First login to get session
	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	loginReq.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	loginW := httptest.NewRecorder()
	adminServer.AdminLoginHandler()(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	// Extract session cookie
	cookies := loginW.Result().Cookies()
	require.Len(t, cookies, 1)

	// Now test stats endpoint with session
	req := httptest.NewRequest(http.MethodGet, "/admin/api/stats", nil)
	req.AddCookie(cookies[0])
	w := httptest.NewRecorder()

	adminServer.AdminStatsHandler()(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response, "uploads")
	require.Contains(t, response, "evaluations")
	require.Contains(t, response, "completed")
	require.Contains(t, response, "avg_time")
}

func TestAdminJobsHandler_Unauthorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}
	server := &httpserver.Server{Cfg: cfg}
	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs", nil)
	w := httptest.NewRecorder()

	adminServer.AdminJobsHandler()(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAdminJobsHandler_Authorized(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}

	// Create a proper server with repositories
	upRepo := createMockUploadRepoForAdmin(t)
	jobRepo := createMockJobRepoWithJobsForAdmin(t)
	resRepo := createMockResultRepoForAdmin(t)
	server := httpserver.NewServer(cfg, usecase.NewUploadService(upRepo), usecase.NewEvaluateService(jobRepo, nil, upRepo), usecase.NewResultService(jobRepo, resRepo), nil, nil, nil, nil)

	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	// First login to get session
	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	loginReq.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	loginW := httptest.NewRecorder()
	adminServer.AdminLoginHandler()(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	// Extract session cookie
	cookies := loginW.Result().Cookies()
	require.Len(t, cookies, 1)

	// Test jobs endpoint with session
	req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs?page=1&limit=10", nil)
	req.AddCookie(cookies[0])
	w := httptest.NewRecorder()

	adminServer.AdminJobsHandler()(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Contains(t, response, "jobs")
	require.Contains(t, response, "pagination")

	jobs, ok := response["jobs"].([]interface{})
	require.True(t, ok)
	require.Greater(t, len(jobs), 0)
}

func TestAdminJobsHandler_Pagination(t *testing.T) {
	cfg := config.Config{AdminUsername: "admin", AdminPassword: "password", AdminSessionSecret: "secret"}

	// Create a proper server with repositories
	upRepo := createMockUploadRepoForAdmin(t)
	jobRepo := createMockJobRepoForAdmin(t)
	resRepo := createMockResultRepoForAdmin(t)
	server := httpserver.NewServer(cfg, usecase.NewUploadService(upRepo), usecase.NewEvaluateService(jobRepo, nil, upRepo), usecase.NewResultService(jobRepo, resRepo), nil, nil, nil, nil)

	adminServer, err := httpserver.NewAdminServer(cfg, server)
	require.NoError(t, err)

	// First login to get session
	loginReq := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	loginReq.Form = map[string][]string{
		"username": {"admin"},
		"password": {"password"},
	}
	loginW := httptest.NewRecorder()
	adminServer.AdminLoginHandler()(loginW, loginReq)
	require.Equal(t, http.StatusOK, loginW.Code)

	// Extract session cookie
	cookies := loginW.Result().Cookies()
	require.Len(t, cookies, 1)

	// Test with different pagination parameters
	testCases := []struct {
		page   string
		limit  string
		expect int
	}{
		{"1", "5", 5},
		{"2", "10", 10},
		{"", "", 10}, // default values
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/jobs", nil)
		if tc.page != "" {
			req.URL.RawQuery = "page=" + tc.page + "&limit=" + tc.limit
		}
		req.AddCookie(cookies[0])
		w := httptest.NewRecorder()

		adminServer.AdminJobsHandler()(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		pagination, ok := response["pagination"].(map[string]interface{})
		require.True(t, ok)
		require.Contains(t, pagination, "page")
		require.Contains(t, pagination, "limit")
	}
}

// Mock creation functions for testing
func createMockUploadRepoForAdmin(t *testing.T) *domainmocks.UploadRepository {
	mockRepo := domainmocks.NewUploadRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("upload-1", nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Upload{ID: "upload-1"}, nil).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByType(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepoForAdmin(t *testing.T) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("job-1", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Job{ID: "job-1"}, nil).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(float64(0), nil).Maybe()
	return mockRepo
}

func createMockJobRepoWithJobsForAdmin(t *testing.T) *domainmocks.JobRepository {
	mockRepo := domainmocks.NewJobRepository(t)
	mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return("job-1", nil).Maybe()
	mockRepo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().Get(mock.Anything, mock.Anything).Return(domain.Job{ID: "job-1"}, nil).Maybe()
	mockRepo.EXPECT().FindByIdempotencyKey(mock.Anything, mock.Anything).Return(domain.Job{}, domain.ErrNotFound).Maybe()
	mockRepo.EXPECT().Count(mock.Anything).Return(int64(2), nil).Maybe()
	mockRepo.EXPECT().CountByStatus(mock.Anything, mock.Anything).Return(int64(1), nil).Maybe()
	mockRepo.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]domain.Job{
		{ID: "job-1", Status: domain.JobCompleted},
		{ID: "job-2", Status: domain.JobProcessing},
	}, nil).Maybe()
	mockRepo.EXPECT().GetAverageProcessingTime(mock.Anything).Return(1.5, nil).Maybe()
	return mockRepo
}

func createMockResultRepoForAdmin(t *testing.T) *domainmocks.ResultRepository {
	mockRepo := domainmocks.NewResultRepository(t)
	mockRepo.EXPECT().Upsert(mock.Anything, mock.Anything).Return(nil).Maybe()
	mockRepo.EXPECT().GetByJobID(mock.Anything, mock.Anything).Return(domain.Result{}, domain.ErrNotFound).Maybe()
	return mockRepo
}
