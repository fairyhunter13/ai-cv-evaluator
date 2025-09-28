//go:build ignore
// Integration tests are disabled in this project. Use E2E tests instead.

package integration

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func Test_Tika_And_Qdrant_Up(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Start Tika
	tikaReq := testcontainers.ContainerRequest{
		Image:        "apache/tika:2.9.0.0",
		ExposedPorts: []string{"9998/tcp"},
		WaitingFor:   wait.ForHTTP("/version").WithPort("9998/tcp").WithStartupTimeout(60 * time.Second),
	}
	tikaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: tikaReq, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tikaC.Terminate(ctx) })

	host, err := tikaC.Host(ctx)
	require.NoError(t, err)
	p, err := tikaC.MappedPort(ctx, "9998")
	require.NoError(t, err)
	url := "http://" + host + ":" + p.Port() + "/version"

	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Get(url)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	_ = resp.Body.Close()

	// Start Qdrant
	qdrReq := testcontainers.ContainerRequest{
		Image:        "qdrant/qdrant:latest",
		ExposedPorts: []string{"6333/tcp"},
		WaitingFor:   wait.ForHTTP("/collections").WithPort("6333/tcp").WithStartupTimeout(90 * time.Second),
	}
	qdrC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: qdrReq, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = qdrC.Terminate(ctx) })

	host2, err := qdrC.Host(ctx)
	require.NoError(t, err)
	p2, err := qdrC.MappedPort(ctx, "6333")
	require.NoError(t, err)
	url2 := "http://" + host2 + ":" + p2.Port() + "/collections"

	resp2, err := cli.Get(url2)
	require.NoError(t, err)
	require.Equal(t, 200, resp2.StatusCode)
	_ = resp2.Body.Close()

	// Start Postgres
	pgReq := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		Env:          map[string]string{"POSTGRES_PASSWORD": "postgres", "POSTGRES_USER": "postgres", "POSTGRES_DB": "app"},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor:   wait.ForLog("database system is ready to accept connections").WithStartupTimeout(90 * time.Second),
	}
	pgC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: pgReq, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })
	pgh, err := pgC.Host(ctx)
	require.NoError(t, err)
	pgp, err := pgC.MappedPort(ctx, "5432")
	require.NoError(t, err)
	dsn := "postgres://postgres:postgres@" + pgh + ":" + pgp.Port() + "/app?sslmode=disable"
	// Verify DB connectivity via pgx stdlib
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	require.Eventually(t, func() bool { return db.Ping() == nil }, 30*time.Second, 1*time.Second)

	// Start Redis
	rdReq := testcontainers.ContainerRequest{
		Image:        "redis:7",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections").WithStartupTimeout(60 * time.Second),
	}
	rdC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: rdReq, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdC.Terminate(ctx) })
	rdh, err := rdC.Host(ctx)
	require.NoError(t, err)
	rdp, err := rdC.MappedPort(ctx, "6379")
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: rdh + ":" + rdp.Port()})
	require.Eventually(t, func() bool { return rdb.Ping(ctx).Err() == nil }, 30*time.Second, 1*time.Second)
}
