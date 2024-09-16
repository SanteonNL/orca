package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func setupNutsNode(t *testing.T, dockerNetworkName string) (string, string) {
	// We use 1 Nuts node, since otherwise the did:web resolving would fail: did:web is resolved over HTTPS,
	// which then requires a reverse proxy with a self-issued certificate, then to be loaded into the Nuts node.
	// To avoid such complexity, we use 1 Nuts node, which then resolves DID documents in its local store.
	println("Starting Nuts node...")
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "nutsfoundation/nuts-node:6.0.0-beta.12",
		Name:         "nutsnode",
		ExposedPorts: []string{"8080/tcp", "8081/tcp"},
		Networks:     []string{dockerNetworkName},
		LogConsumerCfg: &testcontainers.LogConsumerConfig{
			Consumers: []testcontainers.LogConsumer{&testcontainers.StdoutLogConsumer{}},
		},
		Env: map[string]string{
			"NUTS_VERBOSITY":                       "debug",
			"NUTS_STRICTMODE":                      "false",
			"NUTS_HTTP_INTERNAL_ADDRESS":           ":8081",
			"NUTS_AUTH_CONTRACTVALIDATORS":         "dummy",
			"NUTS_URL":                             "http://nutsnode:8080",
			"NUTS_POLICY_DIRECTORY":                "/nuts/policy/",
			"NUTS_DISCOVERY_DEFINITIONS_DIRECTORY": "/nuts/discovery/",
			"NUTS_DISCOVERY_SERVER_IDS":            "dev:HomeMonitoring2024",
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
		WaitingFor: wait.ForHTTP("/"),
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
	publicEndpoint, err := container.PortEndpoint(ctx, "8080/tcp", "http")
	require.NoError(t, err)
	internalEndpoint, err := container.PortEndpoint(ctx, "8081/tcp", "http")
	require.NoError(t, err)
	return publicEndpoint, internalEndpoint
}

func createNutsIdentity(internalAPI string, subject string, ura int, name string, city string, notificationEndpointURL string) error {
	subjectDID, err := createNutsSubject(internalAPI, subject)
	if err != nil {
		return err
	}
	issueNutsURACredential(internalAPI, subjectDID, ura, name, city)
	nutsActivateDiscoveryService(internalAPI, subject, notificationEndpointURL)
	return nil
}

func nutsActivateDiscoveryService(internalAPI string, subject string, notificationEndpointURL string) {
	requestBody, _ := json.Marshal(map[string]interface{}{
		"registrationParameters": map[string]string{
			// fhir-notify should be defined by the use case
			"fhir-notify": notificationEndpointURL,
		},
	})
	httpResponse, err := http.Post(internalAPI+"/internal/discovery/v1/dev:HomeMonitoring2024/"+subject, jsonType, bytes.NewReader(requestBody))
	testHTTPResponse(err, httpResponse, http.StatusOK)
}

// createNutsSubject creates a Nuts subject, returning its preferred DID.
func createNutsSubject(internalAPI string, subject string) (string, error) {
	httpResponse, err := http.Post(internalAPI+"/internal/vdr/v2/subject", "application/json", strings.NewReader(`{"subject":"`+subject+`"}`))
	if err != nil {
		return "", err
	}
	testHTTPResponse(err, httpResponse, http.StatusOK)
	var result map[string]interface{}
	responseData, _ := io.ReadAll(httpResponse.Body)
	if err = json.Unmarshal(responseData, &result); err != nil {
		return "", err
	}
	docs := result["documents"].([]interface{})
	return docs[0].(map[string]interface{})["id"].(string), nil
}

func issueNutsURACredential(internalAPI string, subjectDID string, ura int, name string, city string) {
	// Issue NutsUraCredential
	requestBody := `
	{
	  "@context": [
		"https://www.w3.org/2018/credentials/v1",
		"https://nuts.nl/credentials/2024"
	  ],
	  "type": ["VerifiableCredential", "NutsUraCredential"],
	  "issuer": "` + subjectDID + `",
	  "issuanceDate": "` + time.Now().Format(time.RFC3339) + `",
	  "expirationDate": "` + time.Now().AddDate(1, 0, 0).Format(time.RFC3339) + `",
	  "credentialSubject": {
		"id": "` + subjectDID + `",
		"organization": {
		  "ura": "` + strconv.Itoa(ura) + `",
		  "name": "` + name + `",
		  "city": "` + city + `"
		}
	  }
	}
`
	httpResponse, err := http.Post(internalAPI+"/internal/vcr/v2/issuer/vc", jsonType, strings.NewReader(requestBody))
	testHTTPResponse(err, httpResponse, http.StatusOK)
	// Load into wallet
	httpResponse, err = http.Post(internalAPI+"/internal/vcr/v2/holder/"+subjectDID+"/vc", jsonType, httpResponse.Body)
	testHTTPResponse(err, httpResponse, http.StatusNoContent)
}
