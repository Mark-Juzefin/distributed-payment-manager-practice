//go:build integration
// +build integration

package events_test

import (
	"TestTaskJustPay/pkg/postgres"
	"TestTaskJustPay/testinfra"
	"context"
	"fmt"
	"os"
	"testing"
)

var pool *postgres.Postgres

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := testinfra.NewPostgres(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to start postgres container: %v", err))
	}

	pool = pgContainer.Pool

	code := m.Run()

	pgContainer.Cleanup(ctx)
	os.Exit(code)
}
