package health

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresChecker checks PostgreSQL connectivity.
type PostgresChecker struct {
	pool *pgxpool.Pool
}

// NewPostgresChecker creates a new PostgreSQL health checker.
func NewPostgresChecker(pool *pgxpool.Pool) *PostgresChecker {
	return &PostgresChecker{pool: pool}
}

// Name returns "postgres".
func (c *PostgresChecker) Name() string {
	return "postgres"
}

// Check pings the PostgreSQL database.
func (c *PostgresChecker) Check(ctx context.Context) Result {
	if err := c.pool.Ping(ctx); err != nil {
		return Result{Status: StatusDown, Message: err.Error()}
	}
	return Result{Status: StatusUp}
}
