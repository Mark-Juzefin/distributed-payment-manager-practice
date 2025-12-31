//go:build integration
// +build integration

package testinfra

import (
	"context"
	"fmt"
	"sync"
)

type TestSuite struct {
	Postgres *PostgresContainer
	Kafka    *KafkaContainer
	Wiremock *WiremockContainer
}

type SuiteOptions struct {
	WithKafka    bool
	WithWiremock bool
	MappingsPath string // for Wiremock
}

// NewTestSuite creates all infrastructure for tests
// Containers are started in parallel for speed
func NewTestSuite(ctx context.Context, opts SuiteOptions) (*TestSuite, error) {
	suite := &TestSuite{}
	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	// PostgreSQL (always needed)
	wg.Add(1)
	go func() {
		defer wg.Done()
		pg, err := NewPostgres(ctx)
		if err != nil {
			errCh <- fmt.Errorf("postgres: %w", err)
			return
		}
		suite.Postgres = pg
	}()

	// Kafka (optional)
	if opts.WithKafka {
		wg.Add(1)
		go func() {
			defer wg.Done()
			k, err := NewKafka(ctx)
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
			w, err := NewWiremock(ctx, opts.MappingsPath)
			if err != nil {
				errCh <- fmt.Errorf("wiremock: %w", err)
				return
			}
			suite.Wiremock = w
		}()
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		suite.Cleanup(ctx) // cleanup partially started containers
		return nil, fmt.Errorf("failed to start containers: %v", errs)
	}

	return suite, nil
}

func (s *TestSuite) Cleanup(ctx context.Context) {
	if s.Wiremock != nil {
		s.Wiremock.Cleanup(ctx)
	}
	if s.Kafka != nil {
		s.Kafka.Cleanup(ctx)
	}
	if s.Postgres != nil {
		s.Postgres.Cleanup(ctx)
	}
}
