//go:build integration
// +build integration

package order_eventsink_test

import (
	"TestTaskJustPay/internal/testinfra"
	"TestTaskJustPay/pkg/postgres"
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/base_fixture.sql
var baseFixture string

func applyBaseFixture(t *testing.T, tx postgres.Executor) {
	t.Helper()
	_, err := tx.Exec(context.Background(), baseFixture)
	require.NoError(t, err)
}

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
