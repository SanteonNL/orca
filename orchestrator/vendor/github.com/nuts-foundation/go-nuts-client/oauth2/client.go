package oauth2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var _ http.RoundTripper = &Transport{}

func NewClient(tokenSource TokenSource, scope string, authzServerURL *url.URL) *http.Client {
	return &http.Client{
		Transport: &Transport{
			TokenSource:    tokenSource,
			Scope:          scope,
			AuthzServerURL: authzServerURL,
		},
	}
}

type Transport struct {
	TokenSource         TokenSource
	Scope               string
	UnderlyingTransport http.RoundTripper
	AuthzServerURL      *url.URL
}

func (o *Transport) RoundTrip(httpRequest *http.Request) (*http.Response, error) {
	var err error
	var client http.RoundTripper
	if o.UnderlyingTransport == nil {
		client = http.DefaultTransport
	} else {
		client = o.UnderlyingTransport
	}

	var requestBody []byte = nil
	if httpRequest.Body != nil {
		requestBody, err = io.ReadAll(httpRequest.Body)
		if err != nil {
			return nil, fmt.Errorf("reading request body: %w", err)
		}
	}

	const maxTries = 2
	requestFreshToken := false
	var httpResponse *http.Response
	var errs []error
	for tryNum := 1; tryNum <= maxTries; tryNum++ {
		httpRequestCopy := httpRequest.Clone(httpRequest.Context())
		if requestBody != nil {
			httpRequestCopy.Body = io.NopCloser(bytes.NewReader(requestBody))
		}
		httpResponse, err = o.attemptRequest(client, httpRequestCopy, requestFreshToken)
		if err != nil {
			errs = append(errs, err)
		} else {
			requestFreshToken = false
			if httpResponse.StatusCode == http.StatusUnauthorized {
				// Should be retried with a new token.
				requestFreshToken = true
				continue
			}
			// HTTP response OK (or at least not something we can smartly handle)
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed after %d attempts: %w", maxTries, errors.Join(errs...))
	}
	return httpResponse, nil
}

func (o *Transport) attemptRequest(client http.RoundTripper, httpRequest *http.Request, requestFreshToken bool) (*http.Response, error) {
	token, err := o.requestToken(httpRequest, requestFreshToken)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 token request (resource=%s): %w", httpRequest.URL.String(), err)
	}
	httpRequest.Header.Set("Authorization", fmt.Sprintf("%s %s", token.TokenType, token.AccessToken))
	return client.RoundTrip(httpRequest)
}

func (o *Transport) requestToken(httpRequest *http.Request, noCache bool) (*Token, error) {
	// Use the scope from the request context if available.
	scope := o.Scope
	if ctxScope, ok := httpRequest.Context().Value(withScopeContextKeyInstance).(string); ok {
		scope = ctxScope
	}
	if scope == "" {
		return nil, errors.New("scope is required")
	}

	token, err := o.TokenSource.Token(httpRequest, o.AuthzServerURL, scope, noCache)
	if err != nil {
		return nil, err
	}
	return token, err
}

// WithScope returns a new context with the given OAuth2 scope,
// which will override the default scope set in the OAuth2 client.
func WithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, withScopeContextKeyInstance, scope)
}

type withScopeContextKey struct{}

var withScopeContextKeyInstance = withScopeContextKey{}

type resourceURIContextKeyType struct{}

var resourceURIContextKey = resourceURIContextKeyType{}

// WithResourceURI returns a new context with the given resource URI,
// which will be used to fetch the protected resource metadata when using ProtectedResourceMetadataLocator.
// This is useful when the resource server is not able to provide the protected resource metadata URL in the WWW-Authenticate response header.
// E.g., when an API gateway is used that allows limited control over the response headers.
func WithResourceURI(httpRequest *http.Request, uri string) *http.Request {
	return httpRequest.WithContext(context.WithValue(httpRequest.Context(), resourceURIContextKey, uri))
}

type authzServerURLContextKeyType struct{}

var authzServerURLContextKey = authzServerURLContextKeyType{}

// WithAuthzServerURL returns a new context with the given Authorization Server URL,
// which will override the Authorization Server URL determined by the AuthzServerLocators.
// This is useful when the Authorization Server URL cannot be determined from the response headers,
// or if a fixed Authorization Server URL should be used.
func WithAuthzServerURL(httpRequest *http.Request, authzServerURL *url.URL) *http.Request {
	return httpRequest.WithContext(context.WithValue(httpRequest.Context(), authzServerURLContextKey, authzServerURL))
}
