//go:build integration
// +build integration

package testinfra

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/kafka"
)

type KafkaContainer struct {
	Container     *kafka.KafkaContainer
	Brokers       []string
	OrdersTopic   string
	DisputesTopic string
	OrdersGroup   string
	DisputesGroup string
}

func NewKafka(ctx context.Context) (*KafkaContainer, error) {
	container, err := kafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		kafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start kafka container: %w", err)
	}

	brokers, err := container.Brokers(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get brokers: %w", err)
	}

	// Unique topics and groups per test run
	suffix := uuid.New().String()[:8]
	ordersTopic := fmt.Sprintf("test-orders-%s", suffix)
	disputesTopic := fmt.Sprintf("test-disputes-%s", suffix)

	// Create topics explicitly (so consumers can subscribe before first message)
	if err := createTopic(ctx, container, ordersTopic, 3); err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create orders topic: %w", err)
	}
	if err := createTopic(ctx, container, disputesTopic, 3); err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create disputes topic: %w", err)
	}

	return &KafkaContainer{
		Container:     container,
		Brokers:       brokers,
		OrdersTopic:   ordersTopic,
		DisputesTopic: disputesTopic,
		OrdersGroup:   fmt.Sprintf("test-group-orders-%s", suffix),
		DisputesGroup: fmt.Sprintf("test-group-disputes-%s", suffix),
	}, nil
}

func createTopic(ctx context.Context, c *kafka.KafkaContainer, topic string, partitions int) error {
	// Small retry because Kafka may be "up" but not yet ready for admin ops.
	const attempts = 20
	for i := 0; i < attempts; i++ {
		exitCode, reader, err := c.Exec(ctx, []string{
			"kafka-topics",
			"--bootstrap-server", "localhost:9092",
			"--create",
			"--if-not-exists",
			"--topic", topic,
			"--partitions", fmt.Sprintf("%d", partitions),
			"--replication-factor", "1",
		})
		if err == nil && exitCode == 0 {
			return nil
		}

		// Best-effort: read output for debugging
		var out string
		if reader != nil {
			b, _ := io.ReadAll(reader)
			out = strings.TrimSpace(string(b))
		}

		// Last attempt -> return a useful error
		if i == attempts-1 {
			if err != nil {
				return fmt.Errorf("exec kafka-topics failed: %w; output=%q", err, out)
			}
			return fmt.Errorf("kafka-topics exit=%d; output=%q", exitCode, out)
		}

		time.Sleep(250 * time.Millisecond)
	}

	return fmt.Errorf("unreachable")
}

func (c *KafkaContainer) Cleanup(ctx context.Context) {
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}
