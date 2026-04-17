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

// APIContainer wraps a Docker container running the API service.
type APIContainer struct {
	Container testcontainers.Container
	BaseURL   string // host-mapped URL for test client access
}

// APIContainerConfig holds all configuration needed to start the API container.
type APIContainerConfig struct {
	PgDSN             string // Docker-internal DSN (e.g. postgres://postgres:secret@postgres:5432/payments_test)
	KafkaBrokers      string // Docker-internal broker address (e.g. kafka:29092)
	KafkaTopics       TopicNames
	SilvergateBaseURL string // Docker-internal Wiremock URL (e.g. http://wiremock:8080)
	WebhookMode       string // "kafka" or "sync"
	ProjectRoot       string // path to project root for Dockerfile context
	Network           *NetworkConfig
}

// NewAPIContainer builds and starts the API service container from Dockerfile.api.
func NewAPIContainer(ctx context.Context, cfg APIContainerConfig) (*APIContainer, error) {
	env := map[string]string{
		"PORT":                                 "3000",
		"PG_URL":                               cfg.PgDSN,
		"PG_POOL_MAX":                          "5",
		"LOG_LEVEL":                            "debug",
		"LOG_FORMAT":                           "console",
		"WEBHOOK_MODE":                         cfg.WebhookMode,
		"SILVERGATE_BASE_URL":                  cfg.SilvergateBaseURL,
		"SILVERGATE_SUBMIT_REPRESENTMENT_PATH": "/api/v1/dispute-representments/create",
		"SILVERGATE_CAPTURE_PATH":              "/api/v1/capture",
		"SILVERGATE_AUTH_PATH":                 "/api/v1/auth",
		"HTTP_SILVERGATE_CLIENT_TIMEOUT":       "20s",
		"MERCHANT_ID":                          "merchant_e2e",
	}

	// Add Kafka config when webhook processing is via Kafka
	if cfg.WebhookMode == "kafka" {
		env["KAFKA_BROKERS"] = cfg.KafkaBrokers
		env["KAFKA_ORDERS_TOPIC"] = cfg.KafkaTopics.OrdersTopic
		env["KAFKA_DISPUTES_TOPIC"] = cfg.KafkaTopics.DisputesTopic
		env["KAFKA_ORDERS_CONSUMER_GROUP"] = cfg.KafkaTopics.OrdersGroup
		env["KAFKA_DISPUTES_CONSUMER_GROUP"] = cfg.KafkaTopics.DisputesGroup
		env["KAFKA_ORDERS_DLQ_TOPIC"] = cfg.KafkaTopics.OrdersTopic + ".dlq"
		env["KAFKA_DISPUTES_DLQ_TOPIC"] = cfg.KafkaTopics.DisputesTopic + ".dlq"
		env["KAFKA_PAYMENTS_TOPIC"] = cfg.KafkaTopics.PaymentsTopic
		env["KAFKA_PAYMENTS_CONSUMER_GROUP"] = cfg.KafkaTopics.PaymentsGroup
		env["KAFKA_PAYMENTS_DLQ_TOPIC"] = cfg.KafkaTopics.PaymentsTopic + ".dlq"
	}

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    cfg.ProjectRoot,
			Dockerfile: "Dockerfile.paymanager",
		},
		ExposedPorts: []string{"3000/tcp"},
		Env:          env,
		WaitingFor:   wait.ForHTTP("/health/ready").WithPort("3000/tcp").WithStartupTimeout(120 * time.Second),
	}

	// Apply network config
	if cfg.Network != nil {
		req.Networks = []string{cfg.Network.Name}
		req.NetworkAliases = map[string][]string{
			cfg.Network.Name: {"api"},
		}
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start API container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "3000/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &APIContainer{
		Container: container,
		BaseURL:   baseURL,
	}, nil
}

func (c *APIContainer) Cleanup(ctx context.Context) {
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}
