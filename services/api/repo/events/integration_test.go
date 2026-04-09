//go:build integration

package events_test

import (
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/pkg/testinfra"
	api "TestTaskJustPay/services/api"
	"context"
	"fmt"
	"os"
	"testing"
)

var pool *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "payments_test",
		MigrationFS: api.MIGRATION_FS,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to start postgres container: %v", err))
	}

	pool = pgContainer.Pool

	code := m.Run()

	pgContainer.Cleanup(ctx)
	os.Exit(code)
}
