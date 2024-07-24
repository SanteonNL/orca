package careplancontributor

type Config struct {
	CarePlanService      CarePlanServiceConfig `koanf:"careplanservice"`
	EnrollmentFromConfig EnrollmentFromConfig  `koanf:"enrollmentform"`
	Enabled              bool                  `koanf:"enabled"`
}

type CarePlanServiceConfig struct {
	// URL is the base URL of the CarePlanService at which the CarePlanContributor creates/reads CarePlans.
	URL string `koanf:"url"`
}
type EnrollmentFromConfig struct {
	// URL is the base URL of the EnrollmentFromConfig at which the CarePlanContributor collects minimal data to create a CarePlan and Task at the CarePlanService with embedded/contained resources
	URL string `koanf:"url"`
}
