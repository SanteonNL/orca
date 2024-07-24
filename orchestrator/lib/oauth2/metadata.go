package oauth2

import (
	"context"
	"fmt"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"net/http"
	"net/url"
)

type resourceURIContextKeyType struct{}

var resourceURIContextKey = resourceURIContextKeyType{}

func WithResourceURIContext(ctx context.Context, uri string) context.Context {
	return context.WithValue(ctx, resourceURIContextKey, uri)
}

func WellKnownProtectedResourceMetadataLocator(metadataLoader *oauth2.MetadataLoader, response *http.Response) (*url.URL, error) {
	resourceURI, ok := response.Request.Context().Value(resourceURIContextKey).(string)
	if !ok {
		return nil, nil
	}
	metadataURL, err := url.Parse(resourceURI)
	metadataURL = metadataURL.JoinPath(".well-known/oauth-protected-resource")
	var metadata oauth2.ProtectedResourceMetadata
	if err := metadataLoader.Load(metadataURL.String(), &metadata); err != nil {
		return nil, fmt.Errorf("OAuth2 protected resource metadata fetch failed (url=%s): %w", metadataURL, err)
	}
	if len(metadata.AuthorizationServers) != 1 {
		// TODO: Might have to support more in future
		return nil, fmt.Errorf("expected exactly one authorization server, got %d", len(metadata.AuthorizationServers))
	}
	result, err := url.Parse(metadata.AuthorizationServers[0])
	if err != nil {
		return nil, err
	}
	return result, nil
}
