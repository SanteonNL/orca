package main

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/require"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"io"
	"net/http"
	"strings"
	"testing"
)

const jsonType = "application/json"

func testHTTPResponse(err error, httpResponse *http.Response, expectedStatus int) {
	if err != nil {
		panic(err)
	}
	if httpResponse.StatusCode != expectedStatus {
		responseData, _ := io.ReadAll(httpResponse.Body)
		detail := "Response data:\n----------------\n" + strings.TrimSpace(string(responseData)) + "\n----------------"
		panic(fmt.Sprintf("unexpected status code (status=%s, expected=%d, url=%s)\n%s", httpResponse.Status, expectedStatus, httpResponse.Request.URL, detail))
	}
}

var _ http.RoundTripper = &AuthorizedRoundTripper{}

type AuthorizedRoundTripper struct {
	Value      string
	Underlying http.RoundTripper
}

func (a AuthorizedRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Add("Authorization", a.Value)
	return a.Underlying.RoundTrip(request)
}

func createTenant(nutsInternalAPI string, hapiFHIRClient fhirclient.Client, identifier string, ura int, name string, city string, notificationEndpointURL string) error {
	println("Creating tenant:", identifier)
	if err := createNutsIdentity(nutsInternalAPI, identifier, ura, name, city, notificationEndpointURL); err != nil {
		return fmt.Errorf("could not create Nuts subject: %w", err)
	}
	if err := createFHIRTenant(identifier, ura, hapiFHIRClient); err != nil {
		return fmt.Errorf("could not create FHIR tenant: %w", err)
	}
	return nil
}

func setupDockerNetwork(t *testing.T) (*testcontainers.DockerNetwork, error) {
	dockerNetwork, err := network.New(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := dockerNetwork.Remove(context.Background()); err != nil {
			panic(err)
		}
	})
	return dockerNetwork, err
}
