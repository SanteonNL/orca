package nuts

import (
	"errors"
	"net/url"
)

type Config struct {
	API              APIConfig    `koanf:"api"`
	Public           PublicConfig `koanf:"public"`
	OwnSubject       string       `koanf:"subject"`
	DiscoveryService string       `koanf:"discoveryservice"`
}

func (c Config) Validate() error {
	_, err := url.Parse(c.API.URL)
	if c.OwnSubject == "" {
		return errors.New("invalid/empty Nuts subject")
	}
	if err != nil || c.API.URL == "" {
		return errors.New("invalid Nuts API URL")
	}
	if c.Public.URL == "" {
		return errors.New("invalid/empty Nuts public URL")
	}
	if c.DiscoveryService == "" {
		return errors.New("invalid/empty Discovery Service ID")
	}
	return nil
}

type PublicConfig struct {
	URL string `koanf:"url"`
}

func (c PublicConfig) Parse() *url.URL {
	u, _ := url.Parse(c.URL)
	return u
}

type APIConfig struct {
	URL string `koanf:"url"`
}

func (n APIConfig) Parse() *url.URL {
	u, _ := url.Parse(n.URL)
	return u
}
