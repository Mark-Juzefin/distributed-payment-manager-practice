//go:build integration
// +build integration

package testinfra

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type WiremockContainer struct {
	Container testcontainers.Container
	BaseURL   string
}

func NewWiremock(ctx context.Context, mappingsPath string) (*WiremockContainer, error) {
	absPath, err := filepath.Abs(mappingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	req := testcontainers.ContainerRequest{
		Image:        "wiremock/wiremock:latest",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/__admin/mappings").WithPort("8080/tcp"),
		Cmd:          []string{"--global-response-templating", "--disable-gzip", "--verbose"},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(absPath, "/home/wiremock/mappings"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start wiremock container: %w", err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "8080/tcp")
	baseURL := fmt.Sprintf("http://%s:%s", host, port.Port())

	return &WiremockContainer{
		Container: container,
		BaseURL:   baseURL,
	}, nil
}

func (c *WiremockContainer) Cleanup(ctx context.Context) {
	if c.Container != nil {
		c.Container.Terminate(ctx)
	}
}
