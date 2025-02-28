package main

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupOrchestrator(t *testing.T, dockerNetworkName string, containerName string, nutsSubject string, cpsEnabled bool, cpsFhirBaseUrl string, fhirStoreURL string, questionnaireFhirStoreUrl string, allowUnmanagedFHIROperations bool) *url.URL {
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
			"ORCA_LOGLEVEL":                                     "debug",
			"ORCA_PUBLIC_URL":                                   "http://" + containerName + ":8080",
			"ORCA_NUTS_API_URL":                                 "http://nutsnode:8081",
			"ORCA_NUTS_PUBLIC_URL":                              "http://nutsnode:8080",
			"ORCA_NUTS_SUBJECT":                                 nutsSubject,
			"ORCA_NUTS_DISCOVERYSERVICE":                        "dev:HomeMonitoring2024",
			"ORCA_CAREPLANSERVICE_ENABLED":                      strconv.FormatBool(cpsEnabled),
			"ORCA_CAREPLANSERVICE_FHIR_URL":                     fhirStoreURL,
			"ORCA_CAREPLANSERVICE_ALLOWUNMANAGEDFHIROPERATIONS": strconv.FormatBool(allowUnmanagedFHIROperations),
			"ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL":      cpsFhirBaseUrl,
			"ORCA_CAREPLANCONTRIBUTOR_FHIR_URL":                 fhirStoreURL,
			// HAPI FHIR can only store Questionnaires in the default partition.
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIREFHIR_URL": questionnaireFhirStoreUrl,
			"ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS": "file:///config/fhir/healthcareservices.json,file:///config/fhir/questionnaires.json",
			"ORCA_CAREPLANCONTRIBUTOR_ENABLED":                          "true",
			"ORCA_CAREPLANCONTRIBUTOR_STATICBEARERTOKEN":                "valid",
		},
		Files: []testcontainers.ContainerFile{
			// Questionnaire and HealthcareService bundles required by Task Filler Engine
			{
				HostFilePath:      "../orchestrator/careplancontributor/taskengine/testdata/healthcareservice-bundle.json",
				ContainerFilePath: "/config/fhir/healthcareservices.json",
				FileMode:          0444,
			},
			{
				HostFilePath:      "../orchestrator/careplancontributor/taskengine/testdata/questionnaire-bundle.json",
				ContainerFilePath: "/config/fhir/questionnaires.json",
				FileMode:          0444,
			},
		},
		WaitingFor: wait.ForHTTP("/health"),
	}
	if questionnaireFhirStoreUrl == "" {
		delete(req.Env, "ORCA_CAREPLANCONTRIBUTOR_TASKFILLER_QUESTIONNAIRESYNCURLS")
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
