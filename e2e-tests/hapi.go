package main

import (
	"context"
	"e2e-tests/to"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net/url"
	"strings"
	"testing"
)

func setupHAPI(t *testing.T, dockerNetworkName string) *url.URL {
	println("Starting HAPI FHIR server...")
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Name:         "fhirstore",
		Image:        "hapiproject/hapi:v7.2.0",
		ExposedPorts: []string{"8080/tcp"},
		Networks:     []string{dockerNetworkName},
		Env: map[string]string{
			"hapi.fhir.fhir_version":                                    "R4",
			"hapi.fhir.partitioning.allow_references_across_partitions": "false",
		},
		WaitingFor: wait.ForHTTP("/fhir/DEFAULT/Account"),
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
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
	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	return u.JoinPath("fhir")
}

func createFHIRTenant(name string, id int, fhirClient fhirclient.Client) error {
	println("Creating HAPI FHIR tenant: " + name)
	var tenant fhir.Parameters
	tenant.Parameter = []fhir.ParametersParameter{
		{
			Name:         "id",
			ValueInteger: to.Ptr(id),
		},
		{
			Name:        "name",
			ValueString: to.Ptr(name),
		},
	}
	err := fhirClient.Create(tenant, &tenant, fhirclient.AtPath("DEFAULT/$partition-management-create-partition"))
	if err != nil && strings.Contains(err.Error(), "status=400") {
		// assume it's OK (maybe it already exists)
		return nil
	}
	return err
}
