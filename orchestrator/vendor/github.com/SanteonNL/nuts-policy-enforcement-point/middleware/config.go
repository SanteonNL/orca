package middleware

import (
	"net/http"
	"net/url"
)

type Config struct {
	TokenIntrospectionEndpoint string
	// TokenIntrospectionClient is the HTTP client used to introspect access tokens. If it's not set, http.DefaultClient is used.
	TokenIntrospectionClient *http.Client
	// BaseURL is the base URL of the service that the middleware is protecting.
	// It's used to construct the well-known Protected Resource Metadata URL in case of a 401 Unauthorized response.
	// If it's not set, the middleware will not include the Protected Resource Metadata URL in the response.
	BaseURL *url.URL
}
