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

// IngestContainer wraps a Docker container running the Ingest service.
type IngestContainer struct {
	Container testcontainers.Container
	BaseURL   string // host-mapped URL for test client access
}

// IngestContainerConfig holds all configuration needed to start the Ingest container.
type IngestContainerConfig struct {
	WebhookMode  string // "kafka" or "http"
	KafkaBrokers string // Docker-internal broker address (e.g. kafka:29092)
	KafkaTopics  TopicNames
	APIBaseURL   string // Docker-internal API URL for HTTP mode (e.g. http://api:3000)
	ProjectRoot  string // path to project root for Dockerfile context
	Network      *NetworkConfig
}

// NewIngestContainer builds and starts the Ingest service container from Dockerfile.ingest.
func NewIngestContainer(ctx context.Context, cfg IngestContainerConfig) (*IngestContainer, error) {
	env := map[string]string{
		"PORT":         "3001",
		"LOG_LEVEL":    "debug",
		"LOG_FORMAT":   "console",
		"WEBHOOK_MODE": cfg.WebhookMode,
	}

	switch cfg.WebhookMode {
	case "kafka":
		env["KAFKA_BROKERS"] = cfg.KafkaBrokers
		env["KAFKA_ORDERS_TOPIC"] = cfg.KafkaTopics.OrdersTopic
		env["KAFKA_DISPUTES_TOPIC"] = cfg.KafkaTopics.DisputesTopic
	case "http":
		env["API_BASE_URL"] = cfg.APIBaseURL
		env["API_TIMEOUT"] = "10s"
		env["API_RETRY_ATTEMPTS"] = "3"
		env["API_RETRY_BASE_DELAY"] = "100ms"
		env["API_RETRY_MAX_DELAY"] = "5s"
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    cfg.ProjectRoot,
			Dockerfile: "Dockerfile.ingest",
		},
		ExposedPorts: []string{"3001/tcp"},
		Env:          env,
		WaitingFor:   wait.ForHTTP("/health/ready").WithPort("3001/tcp").WithStartupTimeout(90 * time.Second),
	}

	// Apply network config
	if cfg.Network != nil {
		req.Networks = []string{cfg.Network.Name}
		req.NetworkAliases = map[string][]string{
			cfg.Network.Name: {"ingest"},
		}
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start Ingest container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "3001/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &IngestContainer{
		Container: container,
		BaseURL:   baseURL,
	}, nil
}

func (c *IngestContainer) Cleanup(ctx context.Context) {
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}
