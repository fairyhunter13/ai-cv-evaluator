package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
	domainmocks "github.com/fairyhunter13/ai-cv-evaluator/internal/domain/mocks"
	"github.com/fairyhunter13/ai-cv-evaluator/internal/usecase"
)

func TestUploadService_Count(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		expectedCount int64
		expectedError bool
		setupMock     func(*domainmocks.UploadRepository)
	}{
		{
			name:          "successful count",
			expectedCount: 100,
			expectedError: false,
			setupMock: func(repo *domainmocks.UploadRepository) {
				repo.EXPECT().Count(mock.Anything).Return(int64(100), nil).Once()
			},
		},
		{
			name:          "repository error",
			expectedCount: 0,
			expectedError: true,
			setupMock: func(repo *domainmocks.UploadRepository) {
				repo.EXPECT().Count(mock.Anything).Return(int64(0), errors.New("database error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := domainmocks.NewUploadRepository(t)
			tt.setupMock(repo)
			service := usecase.NewUploadService(repo)
			count, err := service.Count(context.Background())

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}
		})
	}
}

func TestUploadService_CountByType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		uploadType    string
		expectedCount int64
		expectedError bool
		setupMock     func(*domainmocks.UploadRepository)
	}{
		{
			name:          "successful count by type - CV",
			uploadType:    domain.UploadTypeCV,
			expectedCount: 50,
			expectedError: false,
			setupMock: func(repo *domainmocks.UploadRepository) {
				repo.EXPECT().CountByType(mock.Anything, domain.UploadTypeCV).Return(int64(50), nil).Once()
			},
		},
		{
			name:          "successful count by type - Project",
			uploadType:    domain.UploadTypeProject,
			expectedCount: 30,
			expectedError: false,
			setupMock: func(repo *domainmocks.UploadRepository) {
				repo.EXPECT().CountByType(mock.Anything, domain.UploadTypeProject).Return(int64(30), nil).Once()
			},
		},
		{
			name:          "repository error",
			uploadType:    domain.UploadTypeCV,
			expectedCount: 0,
			expectedError: true,
			setupMock: func(repo *domainmocks.UploadRepository) {
				repo.EXPECT().CountByType(mock.Anything, domain.UploadTypeCV).Return(int64(0), errors.New("database error")).Once()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := domainmocks.NewUploadRepository(t)
			tt.setupMock(repo)
			service := usecase.NewUploadService(repo)
			count, err := service.CountByType(context.Background(), tt.uploadType)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}
		})
	}
}
