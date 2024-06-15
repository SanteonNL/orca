package coolfhir

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type ClientCreator func(properties map[string]string) *DefaultFHIRClient

var ClientFactories = map[string]ClientCreator{}

type FHIRClient interface {
	Get(path string, target interface{}) error
	GetQuery(path string, query url.Values, target interface{}) error
}

func NewClient(fhirBaseURL *url.URL, httpClient *http.Client) *DefaultFHIRClient {
	return &DefaultFHIRClient{
		BaseURL:    fhirBaseURL,
		HTTPClient: httpClient,
	}
}

var _ FHIRClient = &DefaultFHIRClient{}

type DefaultFHIRClient struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
}

func (d DefaultFHIRClient) Get(path string, target interface{}) error {
	return d.GetQuery(path, url.Values{}, target)
}

func (d DefaultFHIRClient) GetQuery(path string, query url.Values, target interface{}) error {
	requestURL := d.BaseURL.JoinPath(path)
	if len(query) > 0 {
		requestURL.RawQuery = query.Encode()
	}

	httpRequest, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return err
	}
	httpRequest.Header.Add("Cache-Control", "no-cache")
	httpRequest.Header.Add("Content-Type", "application/fhir+json")
	httpResponse, err := d.HTTPClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("FHIR request failed (url=%s): %w", requestURL.String(), err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return fmt.Errorf("FHIR request failed (url=%s, status=%d)", requestURL.String(), httpResponse.StatusCode)
	}
	data, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Errorf("FHIR response read failed (url=%s): %w", requestURL.String(), err)
	}
	err = json.Unmarshal(data, &target)
	if err != nil {
		return fmt.Errorf("FHIR response unmarshal failed (url=%s): %w", requestURL.String(), err)
	}
	return nil
}
