package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/smartonfhir_backend_adapter/keys"
	"github.com/SanteonNL/orca/smartonfhir_backend_adapter/smart_on_fhir"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

func main() {
	var (
		clientID                   string
		listenAddress              string
		fhirBaseURL                string
		signingKeyFile             string
		signingKeyAzureKeyVaultURL string
		signingKeyAzureKeyName     string
	)
	envVariables := map[string]*string{
		"SOF_BACKEND_ADAPTER_OAUTH_CLIENT_ID": &clientID,
		"SOF_BACKEND_ADAPTER_FHIR_BASEURL":    &fhirBaseURL,
		"SOF_BACKEND_ADAPTER_LISTEN_ADDRESS":  &listenAddress,
	}
	for name, ptr := range envVariables {
		value := os.Getenv(name)
		if value == "" {
			panic(fmt.Sprintf("Missing environment variable: %s", name))
		}
		*ptr = value
	}
	optionalEnvVariables := map[string]*string{
		"SOF_BACKEND_ADAPTER_AZURE_KEYVAULT_URL":       &signingKeyAzureKeyVaultURL,
		"SOF_BACKEND_ADAPTER_SIGNINGKEY_AZURE_KEYNAME": &signingKeyAzureKeyName,
		"SOF_BACKEND_ADAPTER_SIGNINGKEY_FILE":          &signingKeyFile,
	}
	for name, ptr := range optionalEnvVariables {
		*ptr = os.Getenv(name)
	}
	parsedFHIRBaseURL, err := url.Parse(fhirBaseURL)
	if err != nil {
		panic(err)
	}

	// Load Signing Key
	var signingKey keys.SigningKey
	if signingKeyAzureKeyVaultURL != "" {
		signingKey, err = keys.SigningKeyFromAzureKeyVault(signingKeyAzureKeyVaultURL, signingKeyAzureKeyName)
		if err != nil {
			panic(err)
		}
	} else if signingKeyFile != "" {
		signingKey, err = keys.SigningKeyFromJWKFile(signingKeyFile)
		if err != nil {
			panic(err)
		}
	} else {
		panic("Signing Key properties must be set to load from JWK file or Azure Key Vault.")
	}

	log.Info().Msgf("Listening on: %s", listenAddress)
	log.Info().Msgf("Proxying to: %s", fhirBaseURL)
	log.Info().Msgf("OAuth2 client ID: %s", clientID)

	handler, err := create(signingKey, parsedFHIRBaseURL, clientID)
	if err != nil {
		panic(err)
	}
	err = http.ListenAndServe(listenAddress, handler)
	if !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func create(signingKey keys.SigningKey, parsedFHIRBaseURL *url.URL, clientID string) (*httputil.ReverseProxy, error) {
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
	reverseProxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(parsedFHIRBaseURL)
			cleanHeaders(r.Out.Header)
		},
	}
	//reverseProxy.Rewrite = func(request *httputil.ProxyRequest) {
	//	request.SetURL(target)
	//	request.Out.Host = r.In.Host // if desired
	//}
	//reverseProxy.Director = func(request *http.Request) {
	//	// We're proxying to an external system, so we don't want this proxy's caller's headers to be forwarded.
	//	//request.Header.Del("Authorization")
	//	request.Header.Del("X-Forwarded-Host")
	//}
	reverseProxy.Transport = LoggingTransportDecorator{
		RoundTripper: fhirHTTPClient.Transport,
	}
	reverseProxy.ErrorHandler = func(responseWriter http.ResponseWriter, request *http.Request, err error) {
		log.Warn().Err(err).Msgf("Proxy error: %s", sanitizeRequestURL(request.URL).String())
		responseWriter.Header().Add("Content-Type", "application/fhir+json")
		responseWriter.WriteHeader(http.StatusBadGateway)
		diagnostics := "The system tried to proxy the FHIR operation, but an error occurred."
		data, _ := json.Marshal(fhir.OperationOutcome{
			Issue: []fhir.OperationOutcomeIssue{
				{
					Severity:    fhir.IssueSeverityError,
					Diagnostics: &diagnostics,
				},
			},
		})
		_, _ = responseWriter.Write(data)
	}
	return reverseProxy, nil
}

func cleanHeaders(header http.Header) {
	for name, _ := range header {
		switch name {
		case "Content-Type":
			continue
		case "Accept":
			continue
		case "Accept-Encoding":
			continue
		case "User-Agent":
			continue
		case "X-Request-Id":
			// useful for tracing
			continue
		default:
			header.Del(name)
		}
	}
}

type LoggingTransportDecorator struct {
	RoundTripper http.RoundTripper
}

func (d LoggingTransportDecorator) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := d.RoundTripper.RoundTrip(request)
	if err != nil {
		log.Warn().Msgf("Proxy request failed: %s", sanitizeRequestURL(request.URL).String())
	} else {
		log.Info().Msgf("Proxied request: %s", sanitizeRequestURL(request.URL).String())
	}
	return response, err
}

func sanitizeRequestURL(requestURL *url.URL) *url.URL {
	// Query might contain PII (e.g., social security number), so do not log it.
	requestURLWithoutQuery := *requestURL
	requestURLWithoutQuery.RawQuery = ""
	return &requestURLWithoutQuery
}
