package careplancontributor

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
	FrontendConfig  FrontendConfig        `koanf:"frontend"`
	Enabled         bool                  `koanf:"enabled"`
}

type CarePlanServiceConfig struct {
	// URL is the base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans.
	URL string `koanf:"url"`
}
type FrontendConfig struct {
	// URL is the base URL of the frontend for ORCA
	URL string `koanf:"url"`
}
