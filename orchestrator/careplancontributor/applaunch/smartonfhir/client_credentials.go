package smartonfhir

import (
	"context"
	"github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/client"
	"github.com/zitadel/oidc/v3/pkg/client/profile"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"golang.org/x/oauth2"
	"net/http"
	"time"
)

var _ profile.TokenSource = &JWTTokenSource{}

// JWTTokenSource is an implementation of oauth2.TokenSource, used to sign JWT client assertions for
// confidential OAuth2 clients using a key pair to authenticate calls to the token endpoint.
// zitadel/oidc supports this out-of-the-box, but only with private keys that are available in-memory or on-disk.
// We prefer to store our keys in a secure vault (Azure key Vault), so we need our own implementation that supports that.
// TODO: Contribute this to zitadel/oidc
type JWTTokenSource struct {
	ClientID         string
	Audience         []string
	Expiry           time.Duration
	Signer           jose.Signer
	Scopes           []string
	TokenEndpointURL string
	HTTPClient       *http.Client
}

func (j *JWTTokenSource) TokenCtx(ctx context.Context) (*oauth2.Token, error) {
	assertion, err := client.SignedJWTProfileAssertion(j.ClientID, j.Audience, j.Expiry, j.Signer)
	if err != nil {
		return nil, err
	}
	return client.JWTProfileExchange(ctx, oidc.NewJWTProfileGrantRequest(assertion, j.Scopes...), j)
}

func (j *JWTTokenSource) Token() (*oauth2.Token, error) {
	return j.TokenCtx(context.Background())
}

func (j *JWTTokenSource) TokenEndpoint() string {
	return j.TokenEndpointURL
}

func (j *JWTTokenSource) HttpClient() *http.Client {
	return j.HTTPClient
}
