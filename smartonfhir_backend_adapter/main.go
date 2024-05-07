package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/smartonfhir_backend_adapter/smart_on_fhir"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"golang.org/x/oauth2"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	var (
		clientID      string
		listenAddress string
		fhirBaseURL   string
		jwkFile       string
		jwkKeyID      string
	)
	envVariables := map[string]*string{
		"SOF_BACKEND_ADAPTER_OAUTH_CLIENT_ID": &clientID,
		"SOF_BACKEND_ADAPTER_FHIR_BASEURL":    &fhirBaseURL,
		"SOF_BACKEND_ADAPTER_LISTEN_ADDRESS":  &listenAddress,
		"SOF_BACKEND_ADAPTER_JWK_FILE":        &jwkFile,
		"SOF_BACKEND_ADAPTER_JWK_KEYID":       &jwkKeyID,
	}
	for name, ptr := range envVariables {
		value := os.Getenv(name)
		if value == "" {
			panic(fmt.Sprintf("Missing environment variable: %s", name))
		}
		*ptr = value
	}
	parsedFHIRBaseURL, err := url.Parse(fhirBaseURL)
	if err != nil {
		panic(err)
	}

	log.Println("Listening on", listenAddress)
	log.Println("Proxying to", fhirBaseURL)
	log.Println("OAuth2 client ID", clientID)

	handler, err := create(jwkFile, jwkKeyID, parsedFHIRBaseURL, clientID)
	if err != nil {
		log.Fatalln(err)
	}
	err = http.ListenAndServe(listenAddress, handler)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalln(err)
	}
}

func create(jwkFile string, jwkKeyID string, parsedFHIRBaseURL *url.URL, clientID string) (*httputil.ReverseProxy, error) {
	// Load JWK for signing OAuth2 client assertions
	signingKey, err := loadJWK(jwkFile, jwkKeyID)
	if err != nil {
		return nil, err
	}
	// Discovery SMART on FHIR configuration
	smartConfig, err := smart_on_fhir.DiscoverConfiguration(parsedFHIRBaseURL)
	if err != nil {
		return nil, err
	}
	// Create OAuth2 client for authenticating calls to FHIR API
	fhirHTTPClient := oauth2.NewClient(context.Background(), oauth2.ReuseTokenSource(nil, smart_on_fhir.BackendTokenSource{
		OAuth2ASTokenEndpoint: smartConfig.TokenEndpoint,
		ClientID:              clientID,
		SigningKey:            signingKey,
	}))
	// Spin up proxy
	reverseProxy := httputil.NewSingleHostReverseProxy(parsedFHIRBaseURL)
	reverseProxy.Transport = LoggingTransportDecorator{
		RoundTripper: fhirHTTPClient.Transport,
	}
	return reverseProxy, nil
}

func loadJWK(jwkFile string, keyID string) (jwk.Key, error) {
	data, err := os.ReadFile(jwkFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read JWK file %s: %w", jwkFile, err)
	}
	jwkSet, err := jwk.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK file %s: %w", jwkFile, err)
	}
	result, exists := jwkSet.LookupKeyID(keyID)
	if !exists {
		return nil, fmt.Errorf("key with ID %s does not exist in JWK file", keyID)
	}
	return result, nil
}

type LoggingTransportDecorator struct {
	RoundTripper http.RoundTripper
}

func (d LoggingTransportDecorator) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := d.RoundTripper.RoundTrip(request)
	// Query might contain PII (social security number) in case of FHIR Search, so do not log it.
	requestURLWithoutQuery := *request.URL
	requestURLWithoutQuery.RawQuery = ""
	if err != nil {
		log.Println("ERR", requestURLWithoutQuery.String())
	} else {
		log.Println(response.StatusCode, requestURLWithoutQuery.String())
	}
	return response, err
}
