package oidc

type Config struct {
	Enabled bool                    `koanf:"enabled"`
	Clients map[string]ClientConfig `koanf:"clients"`
}

type ClientConfig struct {
	// ID holds the OAuth2 client_id of the registered client.
	ID string `koanf:"id"`
	// RedirectURI holds the URI of the client to which the authorization server will redirect after authorization.
	RedirectURI string `koanf:"redirecturi"`
}
