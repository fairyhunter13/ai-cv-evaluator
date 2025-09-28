package postgres_test

import (
    "context"
    "errors"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/fairyhunter13/ai-cv-evaluator/internal/adapter/repo/postgres"
    "github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

func TestUploadRepo_Create(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        upload  domain.Upload
        setup   func(*poolStub)
        wantErr bool
        errMsg  string
    }{
        {
            name: "successful create with provided ID",
            upload: domain.Upload{
                ID:       "test-123",
                Type:     "cv",
                Text:     "Test CV content",
                Filename: "test.pdf",
                MIME:     "application/pdf",
                Size:     1024,
            },
            setup: func(m *poolStub) {
                m.execErr = nil
            },
            wantErr: false,
        },
        {
            name: "successful create without ID (generates UUID)",
            upload: domain.Upload{
                Type:     "project",
                Text:     "Test project content",
                Filename: "project.docx",
                MIME:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
                Size:     2048,
            },
            setup: func(m *poolStub) { m.execErr = nil },
            wantErr: false,
        },
        {
            name: "database error",
            upload: domain.Upload{
                ID:   "error-123",
                Type: "cv",
                Text: "Error test",
            },
            setup: func(m *poolStub) {
                m.execErr = assert.AnError
            },
            wantErr: true,
            errMsg:  "op=upload.create",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            m := &poolStub{}
            tt.setup(m)

            repo := postgres.NewUploadRepo(m)
            ctx := context.Background()
            
            id, err := repo.Create(ctx, tt.upload)
            
            if tt.wantErr {
                require.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
                assert.Empty(t, id)
            } else {
                require.NoError(t, err)
                assert.NotEmpty(t, id)
                if tt.upload.ID != "" {
                    assert.Equal(t, tt.upload.ID, id)
                }
            }
        })
    }
}

func TestUploadRepo_Get(t *testing.T) {
    t.Parallel()

    fixedTime := time.Now().UTC()

    tests := []struct {
        name    string
        id      string
        setup   func(*poolStub)
        want    domain.Upload
        wantErr bool
        errMsg  string
    }{
        {
            name: "successful get",
            id:   "test-123",
            setup: func(m *poolStub) {
                m.row = rowStub{scan: func(dest ...any) error {
                    *(dest[0].(*string)) = "test-123"
                    *(dest[1].(*string)) = "cv"
                    *(dest[2].(*string)) = "Test CV content"
                    *(dest[3].(*string)) = "test.pdf"
                    *(dest[4].(*string)) = "application/pdf"
                    *(dest[5].(*int64)) = int64(1024)
                    *(dest[6].(*time.Time)) = fixedTime
                    return nil
                }}
            },
            want: domain.Upload{
                ID:        "test-123",
                Type:      "cv",
                Text:      "Test CV content",
                Filename:  "test.pdf",
                MIME:      "application/pdf",
                Size:      1024,
                CreatedAt: fixedTime,
            },
            wantErr: false,
        },
        {
            name: "not found",
            id:   "nonexistent",
            setup: func(m *poolStub) {
                m.row = rowStub{scan: func(_ ...any) error { return assert.AnError }}
            },
            wantErr: true,
            errMsg:  "op=upload.get",
        },
        {
            name: "database error",
            id:   "error-id",
            setup: func(m *poolStub) {
                m.row = rowStub{scan: func(_ ...any) error { return errors.New("connection timeout") }}
            },
            wantErr: true,
            errMsg:  "op=upload.get",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            m := &poolStub{}
            tt.setup(m)

            repo := postgres.NewUploadRepo(m)
            ctx := context.Background()
            
            got, err := repo.Get(ctx, tt.id)
            
            if tt.wantErr {
                require.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
            } else {
                require.NoError(t, err)
                assert.Equal(t, tt.want, got)
            }
        })
    }
}
