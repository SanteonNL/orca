//go:generate mockgen -destination=./azure_oauth_mock.go -package=ehr -source=azure_oauth.go
package ehr

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"strings"
)

type AzureOauthClient interface {
	GetAzureCredential() (*azidentity.DefaultAzureCredential, error)
	GetBearerToken(context context.Context, spt azcore.TokenCredential, endpoint string) (*azcore.AccessToken, error)
}

type AzureOauthClientImpl struct {
}

var newAzureOauthClient = func() (AzureOauthClient, error) {
	return &AzureOauthClientImpl{}, nil
}

func (k *AzureOauthClientImpl) GetBearerToken(context context.Context, spt azcore.TokenCredential, endpoint string) (*azcore.AccessToken, error) {
	scope := getScopeFromEndpoint(endpoint)
	// Acquire a new token and extract expiry
	token, err := spt.GetToken(context, policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func getScopeFromEndpoint(endpoint string) string {
	split := strings.Split(endpoint, ":")
	return "https://" + split[0]
}

func (k *AzureOauthClientImpl) GetAzureCredential() (*azidentity.DefaultAzureCredential, error) {
	return azidentity.NewDefaultAzureCredential(nil)
}
