package coolfhir

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"golang.org/x/oauth2"
	"net/url"
	"time"
)

// NewAzureClient creates a new FHIR client that communicates with an Azure FHIR API.
// It uses the Managed Identity of the Azure environment to authenticate.
func NewAzureClient(fhirBaseURL *url.URL) (fhirclient.Client, error) {
	credential, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
	}
	return newAzureClient(fhirBaseURL, credential, []string{fhirBaseURL.Host + "/.default"})
}

func newAzureClient(fhirBaseURL *url.URL, credential azcore.TokenCredential, scopes []string) (fhirclient.Client, error) {
	ctx := context.Background()
	return fhirclient.New(fhirBaseURL, oauth2.NewClient(ctx, azureTokenSource{
		credential: credential,
		scopes:     scopes,
		ctx:        ctx,
		timeOut:    10 * time.Second,
	})), nil
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
