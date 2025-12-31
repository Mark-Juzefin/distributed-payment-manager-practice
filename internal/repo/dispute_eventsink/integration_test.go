//go:build integration
// +build integration

package dispute_eventsink_test

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

	pgContainer, err := testinfra.NewPostgres(ctx)
	if err != nil {
		panic(fmt.Sprintf("Failed to start postgres container: %v", err))
	}

	pool = pgContainer.Pool

	code := m.Run()

	pgContainer.Cleanup(ctx)
	os.Exit(code)
}
