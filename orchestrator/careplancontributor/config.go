package careplancontributor

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
	Enabled         bool                  `koanf:"enabled"`
}

type CarePlanServiceConfig struct {
	// URL is the base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans.
	URL string `koanf:"url"`
}
