//go:build integration
// +build integration

package testinfra

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type SilvergateContainer struct {
	Container testcontainers.Container
	BaseURL   string
}

type SilvergateContainerConfig struct {
	PgDSN              string // Docker-internal DSN for silvergate DB
	WebhookCallbackURL string // Docker-internal Ingest URL for webhooks
	ProjectRoot        string
	Network            *NetworkConfig
}

func NewSilvergateContainer(ctx context.Context, cfg SilvergateContainerConfig) (*SilvergateContainer, error) {
	env := map[string]string{
		"PORT":                         "3002",
		"SILVERGATE_PG_URL":            cfg.PgDSN,
		"LOG_LEVEL":                    "debug",
		"WEBHOOK_CALLBACK_URL":         cfg.WebhookCallbackURL,
		"ACQUIRER_AUTH_APPROVE_RATE":   "0.85",
		"ACQUIRER_SETTLE_SUCCESS_RATE": "0.90",
		"ACQUIRER_SETTLE_DELAY":        "200ms",
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    cfg.ProjectRoot,
			Dockerfile: "Dockerfile.silvergate",
		},
		ExposedPorts: []string{"3002/tcp"},
		Env:          env,
		WaitingFor:   wait.ForHTTP("/health/ready").WithPort("3002/tcp").WithStartupTimeout(120 * time.Second),
	}

	if cfg.Network != nil {
		req.Networks = []string{cfg.Network.Name}
		req.NetworkAliases = map[string][]string{
			cfg.Network.Name: {"silvergate"},
		}
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start Silvergate container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "3002/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &SilvergateContainer{
		Container: container,
		BaseURL:   baseURL,
	}, nil
}

func (c *SilvergateContainer) Cleanup(ctx context.Context) {
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}
