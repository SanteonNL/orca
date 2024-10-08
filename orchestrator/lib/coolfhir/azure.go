package coolfhir

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// NewAzureFHIRClient creates a new FHIR client that communicates with an Azure FHIR API.
// It uses the Managed Identity of the Azure environment to authenticate.
func NewAzureFHIRClient(fhirBaseURL *url.URL, credential azcore.TokenCredential, scopes []string) fhirclient.Client {
	return fhirclient.New(fhirBaseURL, NewAzureHTTPClient(credential, scopes), Config())
}

func DefaultAzureScope(fhirBaseURL *url.URL) []string {
	return []string{fhirBaseURL.Host + "/.default"}
}

func NewAzureHTTPClient(credential azcore.TokenCredential, scopes []string) *http.Client {
	ctx := context.Background()
	return oauth2.NewClient(ctx, azureTokenSource{
		credential: credential,
		scopes:     scopes,
		ctx:        ctx,
		timeOut:    10 * time.Second,
	})
}

var _ oauth2.TokenSource = &azureTokenSource{}

type azureTokenSource struct {
	credential azcore.TokenCredential
	scopes     []string
	timeOut    time.Duration
	ctx        context.Context
}

func (a azureTokenSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(a.ctx, a.timeOut)
	defer cancel()
	accessToken, err := a.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: a.scopes})
	if err != nil {
		return nil, fmt.Errorf("unable to get OAuth2 access token using Azure credential: %w", err)
	}
	return &oauth2.Token{
		AccessToken: accessToken.Token,
		TokenType:   "Bearer",
		Expiry:      accessToken.ExpiresOn,
	}, nil
}

type ClientConfig struct {
	// BaseURL is the base URL of the FHIR server to connect to.
	BaseURL string `koanf:"url"`
	// Auth is the authentication configuration for the FHIR server.
	Auth AuthConfig `koanf:"auth"`
}

type AuthConfigType string

const (
	Default              AuthConfigType = ""
	AzureManagedIdentity AuthConfigType = "azure-managedidentity"
)

type AuthConfig struct {
	// Type of authentication to use, supported options: azure-managedidentity.
	// Leave empty for no authentication.
	Type         AuthConfigType `koanf:"type"`
	OAuth2Scopes string         `koanf:"scopes"`
}

func NewAuthRoundTripper(config ClientConfig, fhirClientConfig *fhirclient.Config) (http.RoundTripper, fhirclient.Client, error) {
	var transport http.RoundTripper
	var fhirClient fhirclient.Client
	fhirURL, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, nil, err
	}
	switch config.Auth.Type {
	case AzureManagedIdentity:
		opts := &azidentity.ManagedIdentityCredentialOptions{
			ClientOptions: azcore.ClientOptions{},
		}
		var scopes []string
		if config.Auth.OAuth2Scopes != "" {
			scopes = strings.Split(config.Auth.OAuth2Scopes, " ")
		} else {
			scopes = DefaultAzureScope(fhirURL)
		}
		// For UserAssignedManagedIdentity, client ID needs to be explicitly set.
		// Taken from github.com/!azure/azure-sdk-for-go/sdk/azidentity@v1.7.0/default_azure_credential.go:100
		if ID, ok := os.LookupEnv("AZURE_CLIENT_ID"); ok {
			opts.ID = azidentity.ClientID(ID)
		}
		credential, err := azidentity.NewManagedIdentityCredential(opts)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
		}
		httpClient := NewAzureHTTPClient(credential, scopes)
		transport = httpClient.Transport
		fhirClient = fhirclient.New(fhirURL, httpClient, fhirClientConfig)
	case Default:
		transport = http.DefaultTransport
		fhirClient = fhirclient.New(fhirURL, http.DefaultClient, fhirClientConfig)
	default:
		return nil, nil, fmt.Errorf("invalid FHIR authentication type: %s", config.Auth.Type)
	}
	return transport, fhirClient, nil
}
