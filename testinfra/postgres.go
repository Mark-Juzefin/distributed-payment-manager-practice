//go:build integration
// +build integration

package testinfra

import (
	"TestTaskJustPay/internal/api"
	"TestTaskJustPay/pkg/migrations"
	"TestTaskJustPay/pkg/postgres"
	"context"
	"embed"
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

// PostgresConfig allows customizing the test database name and migration FS.
type PostgresConfig struct {
	DBName      string
	MigrationFS embed.FS
}

// NewPostgresWithConfig starts a PostgreSQL container with the given configuration.
func NewPostgresWithConfig(ctx context.Context, cfg PostgresConfig, netCfg ...*NetworkConfig) (*PostgresContainer, error) {
	req := testcontainers.ContainerRequest{
		Image: "pg17-partman:local",
		Env: map[string]string{
			"POSTGRES_USER":     "postgres",
			"POSTGRES_PASSWORD": "secret",
			"POSTGRES_DB":       cfg.DBName,
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForSQL("5432/tcp", "postgres",
			func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://postgres:secret@%s:%s/%s?sslmode=disable", host, port.Port(), cfg.DBName)
			},
		).WithStartupTimeout(60 * time.Second),
	}

	// Apply network config if provided
	if len(netCfg) > 0 && netCfg[0] != nil {
		nc := netCfg[0]
		req.Networks = []string{nc.Name}
		req.NetworkAliases = map[string][]string{
			nc.Name: {"postgres"},
		}
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
	dsn := fmt.Sprintf("postgres://postgres:secret@%s:%s/%s?sslmode=disable", host, port.Port(), cfg.DBName)

	pool, err := postgres.New(dsn, postgres.MaxPoolSize(10))
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	// Apply migrations
	if err := migrations.ApplyMigrations(dsn, cfg.MigrationFS); err != nil {
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

// NewPostgres starts a PostgreSQL container with the default API database and migrations.
func NewPostgres(ctx context.Context, netCfg ...*NetworkConfig) (*PostgresContainer, error) {
	return NewPostgresWithConfig(ctx, PostgresConfig{
		DBName:      "payments_test",
		MigrationFS: api.MIGRATION_FS,
	}, netCfg...)
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
		"TRUNCATE TABLE events, dispute_events, disputes, order_events, orders, evidence CASCADE")
	return err
}
