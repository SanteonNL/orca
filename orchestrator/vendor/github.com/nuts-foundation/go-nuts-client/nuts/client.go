package nuts

import (
	"context"
	"fmt"
	"github.com/nuts-foundation/go-did/vc"
	"github.com/nuts-foundation/go-nuts-client/nuts/iam"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"net/http"
	"net/url"
	"time"
)

type CredentialProvider interface {
	Credentials() []vc.VerifiableCredential
}

// TokenSource returns an oauth2.TokenSource that authenticates to the OAuth2 remote Resource Server with Nuts OAuth2 access tokens.
// It only supports service access tokens (client credentials flow, no OpenID4VP) at the moment.
// It will use the API of a local Nuts node to request the access token.
func TokenSource(nutsAPIURL string, ownDID string) *OAuth2TokenSource {
	return &OAuth2TokenSource{}
}

var _ oauth2.TokenSource = &OAuth2TokenSource{}

type OAuth2TokenSource struct {
	NutsSubject string
	// NutsAPIURL is the base URL of the Nuts node API.
	NutsAPIURL string
	// NutsHttpClient is the HTTP client used to communicate with the Nuts node.
	// If not set, http.DefaultClient is used.
	NutsHttpClient *http.Client
}

func (o OAuth2TokenSource) Token(httpRequest *http.Request, authzServerURL *url.URL, scope string, noCache bool) (*oauth2.Token, error) {
	if o.NutsSubject == "" {
		return nil, fmt.Errorf("ownDID is required")
	}
	var additionalCredentials []vc.VerifiableCredential
	if credsCtx, ok := httpRequest.Context().Value(additionalCredentialsKey).([]vc.VerifiableCredential); ok {
		additionalCredentials = credsCtx
	}
	client, err := iam.NewClient(o.NutsAPIURL)
	if err != nil {
		return nil, err
	}
	// TODO: Might want to support DPoP as well
	var tokenType = iam.ServiceAccessTokenRequestTokenTypeBearer
	// TODO: Is this the right context to use?
	params := iam.RequestServiceAccessTokenParams{}
	if noCache {
		noCacheValue := "no-cache"
		params.CacheControl = &noCacheValue
	}
	response, err := client.RequestServiceAccessToken(httpRequest.Context(), o.NutsSubject, &params, iam.RequestServiceAccessTokenJSONRequestBody{
		AuthorizationServer: authzServerURL.String(),
		Credentials:         &additionalCredentials,
		Scope:               scope,
		TokenType:           &tokenType,
	})
	if err != nil {
		return nil, err
	}
	accessTokenResponse, err := iam.ParseRequestServiceAccessTokenResponse(response)
	if err != nil {
		return nil, err
	}
	if accessTokenResponse.JSON200 == nil {
		return nil, fmt.Errorf("failed service access token response: %s", accessTokenResponse.HTTPResponse.Status)
	}
	var expiry *time.Time
	if accessTokenResponse.JSON200.ExpiresIn != nil {
		expiry = new(time.Time)
		*expiry = time.Now().Add(time.Duration(*accessTokenResponse.JSON200.ExpiresIn) * time.Second)
	}
	return &oauth2.Token{
		AccessToken: accessTokenResponse.JSON200.AccessToken,
		TokenType:   accessTokenResponse.JSON200.TokenType,
		Expiry:      expiry,
	}, nil
}

type additionalCredentialsKeyType struct{}

var additionalCredentialsKey = additionalCredentialsKeyType{}

// WithAdditionalCredentials returns a new context with the additional credentials set.
// They will be provided to the Nuts node when requesting the service access token.
func WithAdditionalCredentials(ctx context.Context, credentials []vc.VerifiableCredential) context.Context {
	return context.WithValue(ctx, additionalCredentialsKey, credentials)
}
