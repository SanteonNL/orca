package main

import (
	"context"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func setupOrchestrator(t *testing.T, dockerNetworkName string, containerName string, nutsSubject string, cpsEnabled bool, cpsFhirBaseUrl string, fhirStoreURL string) *url.URL {
	image := os.Getenv("ORCHESTRATOR_IMAGE")
	pullImage := false
	if image == "" {
		image = "ghcr.io/santeonnl/orca_orchestrator:main"
		pullImage = true
	}
	println("Starting ORCA Orchestrator...")
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:           image,
		Name:            containerName,
		ExposedPorts:    []string{"8080/tcp"},
		Networks:        []string{dockerNetworkName},
		AlwaysPullImage: pullImage,
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
		Env: map[string]string{
			"ORCA_NUTS_API_URL":                            "http://nutsnode:8081",
			"ORCA_NUTS_PUBLIC_URL":                         "http://nutsnode:8080",
			"ORCA_NUTS_SUBJECT":                            nutsSubject,
			"ORCA_CAREPLANSERVICE_ENABLED":                 strconv.FormatBool(cpsEnabled),
			"ORCA_CAREPLANSERVICE_FHIR_URL":                fhirStoreURL,
			"ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL": cpsFhirBaseUrl,
			"ORCA_CAREPLANCONTRIBUTOR_FHIR_URL":            fhirStoreURL,
			"ORCA_CAREPLANCONTRIBUTOR_ENABLED":             "true",
			"ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN":   "valid",
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "config/nuts/discovery/homemonitoring.json",
				ContainerFilePath: "/nuts/discovery/homemonitoring.json",
				FileMode:          0444,
			},
			{
				HostFilePath:      "config/nuts/policy/homemonitoring.json",
				ContainerFilePath: "/nuts/policy/homemonitoring.json",
				FileMode:          0444,
			},
		},
		WaitingFor: wait.ForHTTP("/health"),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
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
	r, _ := url.Parse(endpoint)
	return r
}
