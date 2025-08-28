//go:build integration
// +build integration

package eventsink_test

import (
	"TestTaskJustPay/internal/app"
	"TestTaskJustPay/pkg/postgres"
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

//go:embed testdata/minimal_base.sql
var baseFixture string

//go:embed testdata/pagination_test.sql
var paginationFixture string

//go:embed testdata/filtering_test.sql
var filteringFixture string

//go:embed testdata/edge_cases.sql
var edgeCasesFixture string

func applyBaseFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), baseFixture)
	require.NoError(t, err)
}

func applyPaginationFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), paginationFixture)
	require.NoError(t, err)
}

func applyFilteringFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), filteringFixture)
	require.NoError(t, err)
}

func applyEdgeCasesFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), edgeCasesFixture)
	require.NoError(t, err)
}

var pool *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image: "postgres:13",
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "secret",
			"POSTGRES_DB":       "payments_test",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres",
			func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://postgres:secret@%s:%s/payments_test?sslmode=disable", host, port.Port())
			},
		).WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		panic(err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432/tcp")
	dsn := fmt.Sprintf("postgres://postgres:secret@%s:%s/payments_test?sslmode=disable", host, port.Port())

	pool, err = postgres.New(dsn, postgres.MaxPoolSize(10))
	if err != nil {
		panic(fmt.Sprintf("Failed to create postgres pool: %v", err))
	}

	// Apply migrations
	err = app.ApplyMigrations(dsn, app.MIGRATION_FS)
	if err != nil {
		panic(fmt.Sprintf("Failed to apply migrations: %v", err))
	}

	code := m.Run()

	// orderly shutdown
	pool.Close()
	_ = container.Terminate(ctx)

	os.Exit(code)
}
