package op

import (
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"net/url"
	"time"
)

var _ op.Client = (*Client)(nil)

type Client struct {
	id           string
	secret       string
	loginURL     *url.URL
	redirectURIs []string
}

func (c Client) GetID() string {
	return c.id
}

func (c Client) RedirectURIs() []string {
	return append([]string{}, c.redirectURIs...)
}

func (c Client) PostLogoutRedirectURIs() []string {
	return nil
}

func (c Client) ApplicationType() op.ApplicationType {
	return op.ApplicationTypeWeb
}

func (c Client) AuthMethod() oidc.AuthMethod {
	// TODO: change to client_secret (Basic/Post) or private_key_jwt
	return oidc.AuthMethodNone
}

func (c Client) ResponseTypes() []oidc.ResponseType {
	return []oidc.ResponseType{
		oidc.ResponseTypeCode,
	}
}

func (c Client) GrantTypes() []oidc.GrantType {
	return []oidc.GrantType{
		oidc.GrantTypeCode,
	}
}

func (c Client) LoginURL(authRequestID string) string {
	result := *c.loginURL
	query := result.Query()
	query.Add("authRequestID", authRequestID)
	result.RawQuery = query.Encode()
	return result.String()
}

func (c Client) AccessTokenType() op.AccessTokenType {
	return op.AccessTokenTypeBearer
}

func (c Client) IDTokenLifetime() time.Duration {
	return time.Minute * 15
}

func (c Client) DevMode() bool {
	return !globals.StrictMode
}

func (c Client) RestrictAdditionalIdTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		return scopes
	}
}

func (c Client) RestrictAdditionalAccessTokenScopes() func(scopes []string) []string {
	return func(scopes []string) []string {
		return scopes
	}
}

func (c Client) IsScopeAllowed(scope string) bool {
	switch scope {
	case oidc.ScopeOpenID, oidc.ScopeProfile, oidc.ScopeEmail, ScopePatient:
		return true
	default:
		return false
	}
}

func (c Client) IDTokenUserinfoClaimsAssertion() bool {
	return true
}

func (c Client) ClockSkew() time.Duration {
	return time.Second * 5
}
