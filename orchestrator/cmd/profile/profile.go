package profile

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

// FHIRNotificationURLEndpointName is the name of the endpoint in the CSD that contains the FHIR Notification URL.
const FHIRNotificationURLEndpointName = "fhirNotificationURL"

// FHIRBaseURLEndpointName is the name of the endpoint in the CSD that contains the FHIR Base URL.
const FHIRBaseURLEndpointName = "fhirBaseURL"

type Provider interface {
	// Authenticator returns a middleware http.HandlerFunc that authenticates the caller according to the profile's authentication method.
	// It sets the authenticate user as auth.Principal in the request context.
	// The resourceServerURL is the external base URL of the resource being accessed.
	Authenticator(resourceServerURL *url.URL, fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request)
	// HttpClient returns an HTTP Client that can be used to perform Shared Care Planning transactions at a Care Plan Contributor or Care Plan Service.
	// The HTTP client handles acquiring the required authentication credentials (e.g. OAuth2 access tokens).
	HttpClient(ctx context.Context, serverIdentity fhir.Identifier) (*http.Client, error)
	// CsdDirectory returns the directory service for finding endpoints and organizations through Care Service Discovery (IHE-CSD).
	CsdDirectory() csd.Directory
	// Identities returns the identities of the local tenant (e.g., a care organization).
	Identities(ctx context.Context) ([]fhir.Organization, error)
	CapabilityStatement(cp *fhir.CapabilityStatement)
}
