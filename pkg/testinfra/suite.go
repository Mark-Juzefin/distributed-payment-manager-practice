//go:build integration
// +build integration

package testinfra

import (
	"context"
	"embed"
	"fmt"
	"sync"
)

type TestSuite struct {
	Postgres     *PostgresContainer
	SilvergatePG *PostgresContainer
	Kafka        *KafkaContainer
	Wiremock     *WiremockContainer
	Silvergate   *SilvergateContainer
	API          *APIContainer
	Ingest       *IngestContainer
	Network      *NetworkConfig
}

type SuiteOptions struct {
	WithKafka      bool
	WithWiremock   bool
	WithSilvergate bool
	MappingsPath   string // for Wiremock

	// E2E options: when WithE2E is true, API and Ingest containers are started
	WithE2E     bool
	ProjectRoot string // path to project root (for Dockerfile context)

	// MigrationFS is the embedded filesystem containing SQL migrations for the test database.
	MigrationFS embed.FS

	// SilvergateMigrationFS is the embedded filesystem for Silvergate's database migrations.
	SilvergateMigrationFS embed.FS
}

// NewTestSuite creates all infrastructure for tests.
// Containers are started in parallel for speed.
// When WithE2E is true, a Docker network is created and API/Ingest containers are started.
func NewTestSuite(ctx context.Context, opts SuiteOptions) (*TestSuite, error) {
	suite := &TestSuite{}

	// For E2E mode, create a shared Docker network first
	var netCfg *NetworkConfig
	if opts.WithE2E {
		var err error
		netCfg, err = CreateNetwork(ctx)
		if err != nil {
			return nil, fmt.Errorf("network: %w", err)
		}
		suite.Network = netCfg
	}

	// Phase 1: Start infrastructure containers in parallel
	var wg sync.WaitGroup
	errCh := make(chan error, 4)

	// PostgreSQL (always needed)
	wg.Add(1)
	go func() {
		defer wg.Done()
		pg, err := NewPostgresWithConfig(ctx, PostgresConfig{
			DBName:      "payments_test",
			MigrationFS: opts.MigrationFS,
		}, netCfg)
		if err != nil {
			errCh <- fmt.Errorf("postgres: %w", err)
			return
		}
		suite.Postgres = pg
	}()

	// Silvergate PostgreSQL (optional, separate DB)
	if opts.WithSilvergate {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sgPG, err := NewPostgresWithConfig(ctx, PostgresConfig{
				DBName:       "silvergate_test",
				MigrationFS:  opts.SilvergateMigrationFS,
				NetworkAlias: "silvergate-db",
			}, netCfg)
			if err != nil {
				errCh <- fmt.Errorf("silvergate postgres: %w", err)
				return
			}
			suite.SilvergatePG = sgPG
		}()
	}

	// Kafka (optional)
	if opts.WithKafka {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, err := NewKafka(ctx, netCfg)
			if err != nil {
				errCh <- fmt.Errorf("kafka: %w", err)
				return
			}
			suite.Kafka = k
		}()
	}

	// Wiremock (optional)
	if opts.WithWiremock {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := NewWiremock(ctx, opts.MappingsPath, netCfg)
			if err != nil {
				errCh <- fmt.Errorf("wiremock: %w", err)
				return
			}
			suite.Wiremock = w
		}()
	}

	wg.Wait()
	close(errCh)

	// Collect errors from infra phase
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		suite.Cleanup(ctx)
		return nil, fmt.Errorf("failed to start infrastructure containers: %v", errs)
	}

	// Phase 2: Start E2E service containers (depends on infra being ready)
	if opts.WithE2E {
		if err := suite.startServiceContainers(ctx, opts); err != nil {
			suite.Cleanup(ctx)
			return nil, err
		}
	}

	return suite, nil
}

// startServiceContainers starts API and Ingest containers after infra is ready.
func (s *TestSuite) startServiceContainers(ctx context.Context, opts SuiteOptions) error {
	// Build internal DSN for Docker network
	pgDSN := "postgres://postgres:secret@postgres:5432/payments_test?sslmode=disable"

	// Start Silvergate service container if Silvergate DB is available
	if s.SilvergatePG != nil {
		sgDSN := "postgres://postgres:secret@silvergate-db:5432/silvergate_test?sslmode=disable"
		// Silvergate sends webhooks to Ingest (which will be started below at "ingest:3001")
		sgCfg := SilvergateContainerConfig{
			PgDSN:              sgDSN,
			WebhookCallbackURL: "http://ingest:3001/webhooks/silvergate",
			ProjectRoot:        opts.ProjectRoot,
			Network:            s.Network,
		}
		sgContainer, err := NewSilvergateContainer(ctx, sgCfg)
		if err != nil {
			return fmt.Errorf("silvergate container: %w", err)
		}
		s.Silvergate = sgContainer
	}

	// Determine Silvergate URL (Docker-internal)
	silvergateURL := ""
	if s.Silvergate != nil {
		silvergateURL = "http://silvergate:3002"
	} else if s.Wiremock != nil {
		silvergateURL = "http://wiremock:8080"
	}

	// Kafka internal brokers (Docker DNS alias)
	kafkaBrokers := ""
	var topicNames TopicNames
	if s.Kafka != nil {
		// The Kafka container's BROKER listener runs on port 9092 (Docker-internal).
		// Other containers on the same Docker network reach it via the "kafka" DNS alias.
		kafkaBrokers = "kafka:9092"
		topicNames = s.Kafka.TopicConfig()
	}

	// Start API container
	apiCfg := APIContainerConfig{
		PgDSN:             pgDSN,
		KafkaBrokers:      kafkaBrokers,
		KafkaTopics:       topicNames,
		SilvergateBaseURL: silvergateURL,
		WebhookMode:       "kafka",
		ProjectRoot:       opts.ProjectRoot,
		Network:           s.Network,
	}

	apiContainer, err := NewAPIContainer(ctx, apiCfg)
	if err != nil {
		return fmt.Errorf("api container: %w", err)
	}
	s.API = apiContainer

	// Start Ingest container
	ingestCfg := IngestContainerConfig{
		WebhookMode:  "kafka",
		KafkaBrokers: kafkaBrokers,
		KafkaTopics:  topicNames,
		APIBaseURL:   "http://api:3000", // Docker-internal URL
		ProjectRoot:  opts.ProjectRoot,
		Network:      s.Network,
	}

	ingestContainer, err := NewIngestContainer(ctx, ingestCfg)
	if err != nil {
		return fmt.Errorf("ingest container: %w", err)
	}
	s.Ingest = ingestContainer

	return nil
}

func (s *TestSuite) Cleanup(ctx context.Context) {
	if s.Ingest != nil {
		s.Ingest.Cleanup(ctx)
	}
	if s.API != nil {
		s.API.Cleanup(ctx)
	}
	if s.Silvergate != nil {
		s.Silvergate.Cleanup(ctx)
	}
	if s.SilvergatePG != nil {
		s.SilvergatePG.Cleanup(ctx)
	}
	if s.Wiremock != nil {
		s.Wiremock.Cleanup(ctx)
	}
	if s.Kafka != nil {
		s.Kafka.Cleanup(ctx)
	}
	if s.Postgres != nil {
		s.Postgres.Cleanup(ctx)
	}
	if s.Network != nil {
		s.Network.Cleanup(ctx)
	}
}
