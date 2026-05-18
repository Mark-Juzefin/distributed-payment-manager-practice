//go:build integration

package productrepo_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/pkg/testinfra"
	silvergate "TestTaskJustPay/services/silvergate"
)

var pg *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()
	pgContainer, err := testinfra.NewPostgresWithConfig(ctx, testinfra.PostgresConfig{
		DBName:      "silvergate_product_test",
		MigrationFS: silvergate.MigrationFS(),
		Image:       "postgres:17",
	})
	if err != nil {
		panic(fmt.Sprintf("postgres: %v", err))
	}
	pg = pgContainer.Pool
	code := m.Run()
	pgContainer.Cleanup(ctx)
	os.Exit(code)
}
