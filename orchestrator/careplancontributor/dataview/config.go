package dataview

type Config struct {
	FhirUrl string `koanf:"fhirurl"` // An "external" FHIR server URL that is used to view data
	Enabled bool   `koanf:"enabled"`
}
