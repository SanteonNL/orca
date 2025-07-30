package tenants

import (
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"net/url"
)

type Properties struct {
	ID       string
	Nuts     NutsProperties            `koanf:"nuts"`
	ChipSoft ChipSoftProperties        `koanf:"chipsoft"`
	Demo     DemoProperties            `koanf:"demo"`
	CPS      CarePlanServiceProperties `koanf:"cps"`
}

type NutsProperties struct {
	// Subject is the subject identifier used by Nuts for this care organization, e.g.
	Subject string `koanf:"subject"`
}

type ChipSoftProperties struct {
	// OrganizationID is the ID used by ChipSoft to identify this care organization, e.g. 2.16.840.1.113883.2.4.3.124.8.50.26.03
	OrganizationID string `koanf:"organizationid"`
}

type DemoProperties struct {
	// FHIR specifies the connection to the FHIR API of the demo EHR.
	FHIR coolfhir.ClientConfig `koanf:"fhir"`
}

type CarePlanServiceProperties struct {
	// FHIR specifies the connection to the Care Plan Service FHIR API.
	// It's required if the Care Plan Service is enabled.
	FHIR coolfhir.ClientConfig `koanf:"fhir"`
}

func (c Properties) URL(baseURL *url.URL, spec URLSpec) *url.URL {
	return spec(c.ID, baseURL)
}

type URLSpec func(tenantID string, baseURL *url.URL) *url.URL
