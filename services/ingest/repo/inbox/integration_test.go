//go:build integration

package inbox_test

import (
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/pkg/testinfra"
	ingest "TestTaskJustPay/services/ingest"
	"context"
	"fmt"
	"os"
	"testing"
)

var pool *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "ingest_test",
		MigrationFS: ingest.MigrationFS,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to start postgres container: %v", err))
	}

	pool = pgContainer.Pool

	code := m.Run()

	pgContainer.Cleanup(ctx)
	os.Exit(code)
}
