package coolfhir

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type ClientCreator func(properties map[string]string) *DefaultClient

var ClientFactories = map[string]ClientCreator{}

type Client interface {
	Read(path string, target any, opts ...Option) error
	Create(path string, resource any, result any) error
	Update(path string, resource any, result any) error
}

func NewClient(fhirBaseURL *url.URL, httpClient *http.Client) *DefaultClient {
	return &DefaultClient{
		BaseURL:    fhirBaseURL,
		HTTPClient: httpClient,
	}
}

var _ Client = &DefaultClient{}

type DefaultClient struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
}

func (d DefaultClient) Read(path string, target any, opts ...Option) error {
	httpRequest, err := http.NewRequest(http.MethodGet, d.resourceURL(path).String(), nil)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(httpRequest)
	}
	httpRequest.Header.Add("Cache-Control", "no-cache")
	return d.doRequest(httpRequest, target)
}

func (d DefaultClient) Create(path string, resource any, result any) error {
	data, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequest(http.MethodPost, d.resourceURL(path).String(), io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return err
	}
	httpRequest.Header.Add("Content-Type", "application/fhir+json")
	return d.doRequest(httpRequest, result)
}

func (d DefaultClient) Update(path string, resource any, result any) error {
	data, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequest(http.MethodPut, d.resourceURL(path).String(), io.NopCloser(bytes.NewReader(data)))
	if err != nil {
		return err
	}
	httpRequest.Header.Add("Content-Type", "application/fhir+json")
	return d.doRequest(httpRequest, result)
}

func (d DefaultClient) resourceURL(path string) *url.URL {
	return d.BaseURL.JoinPath(path)
}

func (d DefaultClient) doRequest(httpRequest *http.Request, target any) error {
	httpResponse, err := d.HTTPClient.Do(httpRequest)
	if err != nil {
		return fmt.Errorf("FHIR request failed (url=%s): %w", httpRequest.URL.String(), err)
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return fmt.Errorf("FHIR request failed (url=%s, status=%d)", httpRequest.URL.String(), httpResponse.StatusCode)
	}
	data, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Errorf("FHIR response read failed (url=%s): %w", httpRequest.URL.String(), err)
	}
	// TODO: Handle errornous responses (OperationOutcome?)
	err = json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("FHIR response unmarshal failed (url=%s): %w", httpRequest.URL.String(), err)
	}
	return nil
}

type Option func(r *http.Request)

func QueryParam(key, value string) Option {
	return func(r *http.Request) {
		q := r.URL.Query()
		q.Add(key, value)
		r.URL.RawQuery = q.Encode()
	}
}
