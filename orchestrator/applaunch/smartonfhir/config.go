package smartonfhir

type Config struct {
	ClientID     string `koanf:"clientid"`
	ClientSecret string `koanf:"clientsecret"`
	RedirectURI  string `koanf:"redirecturi"`
	Scope        string `koanf:"scope"`
}
