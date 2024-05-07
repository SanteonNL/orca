package smart_on_fhir

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Configuration represents the discovered SMART on FHIR configuration for the SMART on FHIR client.
// See https://build.fhir.org/ig/HL7/smart-app-launch/conformance.html#metadata
type Configuration struct {
	TokenEndpoint         string `json:"token_endpoint"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
}

// DiscoverConfiguration retrieves the SMART on FHIR configuration from the given FHIR base URL.
// It is expected to be found at `/.well-known/smart-configuration`.
// See https://build.fhir.org/ig/HL7/smart-app-launch/app-launch.html#retrieve-well-knownsmart-configuration
func DiscoverConfiguration(fhirBaseURL *url.URL) (*Configuration, error) {
	discoveryURL := fhirBaseURL.JoinPath("/.well-known/smart-configuration")
	httpRequest, _ := http.NewRequest("GET", discoveryURL.String(), nil)
	httpRequest.Header.Set("Accept", "application/json")
	response, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve SMART on FHIR configuration: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024)) // 1mb
	if err != nil {
		return nil, fmt.Errorf("failed to read SMART on FHIR configuration: %w", err)
	}
	if c := response.StatusCode; c < 200 || c > 299 {
		return nil, fmt.Errorf("failed to retrieve SMART on FHIR configuration: %s", body)
	}
	var configuration Configuration
	if err := json.Unmarshal(body, &configuration); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SMART on FHIR configuration: %w", err)
	}
	return &configuration, nil
}
