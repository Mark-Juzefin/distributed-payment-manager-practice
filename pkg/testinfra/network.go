//go:build integration
// +build integration

package testinfra

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
)

// NetworkConfig holds Docker network information for container-to-container communication.
// When nil is passed to container constructors, containers run in standalone mode (backward compatible).
type NetworkConfig struct {
	Network *testcontainers.DockerNetwork
	Name    string
}

// CreateNetwork creates a shared Docker network for E2E test containers.
func CreateNetwork(ctx context.Context) (*NetworkConfig, error) {
	net, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker network: %w", err)
	}

	return &NetworkConfig{
		Network: net,
		Name:    net.Name,
	}, nil
}

// Cleanup removes the Docker network.
func (n *NetworkConfig) Cleanup(ctx context.Context) {
	if n != nil && n.Network != nil {
		_ = n.Network.Remove(ctx)
	}
}
