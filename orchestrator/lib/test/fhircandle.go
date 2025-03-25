package test

import (
	"context"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/url"
	"testing"
)

func SetupFHIRCandle(t *testing.T) *url.URL {
	t.Log("Setting up FHIR Candle...")
	ctx := context.Background()
	req := tc.ContainerRequest{
		Image:        "ghcr.io/fhir/fhir-candle:latest",
		ExposedPorts: []string{"5826/tcp"},
		WaitingFor:   wait.ForHTTP("/fhir/r4/Task"),
	}
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			panic(err)
		}
	})
	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(t, err)
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	t.Log("FHIR Candle is ready.")
	return u.JoinPath("fhir", "r4")
}
