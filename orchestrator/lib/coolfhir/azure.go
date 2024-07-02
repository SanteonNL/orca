package coolfhir

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"time"
)

// NewAzureFHIRClient creates a new FHIR client that communicates with an Azure FHIR API.
// It uses the Managed Identity of the Azure environment to authenticate.
func NewAzureFHIRClient(fhirBaseURL *url.URL, credential azcore.TokenCredential) fhirclient.Client {
	return fhirclient.New(fhirBaseURL, NewAzureHTTPClient(credential, DefaultAzureScope(fhirBaseURL)), Config())
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
