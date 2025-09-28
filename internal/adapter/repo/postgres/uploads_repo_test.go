package postgres_test

import (
    "context"
    "testing"
    "time"

    pgxmock "github.com/pashagolub/pgxmock"
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
        setup   func(pgxmock.PgxPool)
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
            setup: func(m pgxmock.PgxPool) {
                m.ExpectExec("INSERT INTO uploads").
                    WithArgs("test-123", "cv", "Test CV content", "test.pdf", "application/pdf", int64(1024), pgxmock.AnyArg()).
                    WillReturnResult(pgxmock.NewResult("INSERT", 1))
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
            setup: func(m pgxmock.PgxPool) {
                m.ExpectExec("INSERT INTO uploads").
                    WithArgs(pgxmock.AnyArg(), "project", "Test project content", "project.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", int64(2048), pgxmock.AnyArg()).
                    WillReturnResult(pgxmock.NewResult("INSERT", 1))
            },
            wantErr: false,
        },
        {
            name: "database error",
            upload: domain.Upload{
                ID:   "error-123",
                Type: "cv",
                Text: "Error test",
            },
            setup: func(m pgxmock.PgxPool) {
                m.ExpectExec("INSERT INTO uploads").
                    WithArgs("error-123", "cv", "Error test", "", "", int64(0), pgxmock.AnyArg()).
                    WillReturnError(assert.AnError)
            },
            wantErr: true,
            errMsg:  "op=upload.create",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            m, err := pgxmock.NewPool()
            require.NoError(t, err)
            defer m.Close()
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
            require.NoError(t, m.ExpectationsWereMet())
        })
    }
}

func TestUploadRepo_Get(t *testing.T) {
    t.Parallel()

    fixedTime := time.Now().UTC()

    tests := []struct {
        name    string
        id      string
        setup   func(pgxmock.PgxPool)
        want    domain.Upload
        wantErr bool
        errMsg  string
    }{
        {
            name: "successful get",
            id:   "test-123",
            setup: func(m pgxmock.PgxPool) {
                rows := pgxmock.NewRows([]string{"id","type","text","filename","mime","size","created_at"}).
                    AddRow("test-123","cv","Test CV content","test.pdf","application/pdf", int64(1024), fixedTime)
                m.ExpectQuery(`SELECT id, type, text, filename, mime, size, created_at FROM uploads WHERE id=\$1`).
                    WithArgs("test-123").
                    WillReturnRows(rows)
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
            setup: func(m pgxmock.PgxPool) {
                m.ExpectQuery(`SELECT id, type, text, filename, mime, size, created_at FROM uploads WHERE id=\$1`).
                    WithArgs("nonexistent").
                    WillReturnError(assert.AnError)
            },
            wantErr: true,
            errMsg:  "op=upload.get",
        },
        {
            name: "database error",
            id:   "error-id",
            setup: func(m pgxmock.PgxPool) {
                m.ExpectQuery(`SELECT id, type, text, filename, mime, size, created_at FROM uploads WHERE id=\$1`).
                    WithArgs("error-id").
                    WillReturnError(assert.AnError)
            },
            wantErr: true,
            errMsg:  "op=upload.get",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            m, err := pgxmock.NewPool()
            require.NoError(t, err)
            defer m.Close()
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
            require.NoError(t, m.ExpectationsWereMet())
        })
    }
}
