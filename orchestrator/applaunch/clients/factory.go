package clients

import (
	"net/http"
	"net/url"
)

type ClientProperties struct {
	BaseURL *url.URL
	Client  http.RoundTripper
}

type ClientCreator func(properties map[string]string) ClientProperties

var Factories = map[string]ClientCreator{}
