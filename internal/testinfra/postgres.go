//go:build integration
// +build integration

package testinfra

import (
	"TestTaskJustPay/internal/app"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"fmt"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
	Container testcontainers.Container
	Pool      *postgres.Postgres
	DSN       string
}

func NewPostgres(ctx context.Context) (*PostgresContainer, error) {
	req := testcontainers.ContainerRequest{
		Image: "pg17-partman:local",
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
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432/tcp")
	dsn := fmt.Sprintf("postgres://postgres:secret@%s:%s/payments_test?sslmode=disable", host, port.Port())

	pool, err := postgres.New(dsn, postgres.MaxPoolSize(10))
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Apply migrations
	if err := app.ApplyMigrations(dsn, app.MIGRATION_FS); err != nil {
		pool.Close()
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	return &PostgresContainer{
		Container: container,
		Pool:      pool,
		DSN:       dsn,
	}, nil
}

func (c *PostgresContainer) Cleanup(ctx context.Context) {
	if c.Pool != nil {
		c.Pool.Close()
	}
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}

// Truncate clears all tables (for isolation between tests)
func (c *PostgresContainer) Truncate(ctx context.Context) error {
	_, err := c.Pool.Pool.Exec(ctx,
		"TRUNCATE TABLE dispute_events, disputes, order_events, orders, evidence CASCADE")
	return err
}
