package tenants

import "github.com/SanteonNL/orca/orchestrator/lib/coolfhir"

type Properties struct {
	ID string
	// TODO: Add "CPSURL" here when we add tenants to the ORCA URLs.
	// ChipSoftOrgID is the ID used by ChipSoft for this care organization, e.g. 2.16.840.1.113883.2.4.3.124.8.50.26.03
	ChipSoftOrgID string `koanf:"chipsoftorgid"`
	NutsSubject   string `koanf:"nutssubject"`
	// CPSFHIR specifies the connection to the Care Plan Service FHIR API.
	// It's required if the Care Plan Service is enabled.
	CPSFHIR coolfhir.ClientConfig `koanf:"cpsfhir"`
}
