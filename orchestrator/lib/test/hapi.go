package test

import (
	"context"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/url"
	"testing"
	"time"
)

// File for test helpers that can be shared between CPS and CPC

func SetupHAPI(t *testing.T) *url.URL {
	ctx := context.Background()
	req := tc.ContainerRequest{
		Image:        "hapiproject/hapi:v7.2.0",
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"hapi.fhir.fhir_version":              "R4",
			"hapi.fhir.allow_external_references": "true",
		},
		WaitingFor: wait.ForHTTP("/fhir/Task").WithStartupTimeout(2 * time.Minute),
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
	return u.JoinPath("fhir")
}
