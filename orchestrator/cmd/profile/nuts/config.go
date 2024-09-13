package nuts

import "net/url"

type Config struct {
	API        APIConfig    `koanf:"api"`
	Public     PublicConfig `koanf:"public"`
	OwnSubject string       `koanf:"subject"`
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
