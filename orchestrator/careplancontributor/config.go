package careplancontributor

type Config struct {
	CarePlanService CarePlanServiceConfig `koanf:"careplanservice"`
}

type CarePlanServiceConfig struct {
	// URL is the base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans.
	URL string `koanf:"url"`
}
