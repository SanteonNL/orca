package profile

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

type Provider interface {
	// Authenticator returns a middleware http.HandlerFunc that authenticates the caller according to the profile's authentication method.
	// It sets the authenticate user as auth.Principal in the request context.
	// The resourceServerURL is the external base URL of the resource being accessed.
	Authenticator(resourceServerURL *url.URL, fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request)
	// RegisterHTTPHandlers register's the Profile's custom HTTP handlers on the given router.
	// The resourceServerURL and basePath are there to support multiple contexts (e.g. CarePlanContributor and CarePlanService), for instance:
	// resourceServerURL: https://example.com/cpc
	// basePath: /cpc
	RegisterHTTPHandlers(basePath string, resourceServerURL *url.URL, mux *http.ServeMux)
	// HttpClient returns an HTTP Client that can be used to perform Shared Care Planning transactions at a Care Plan Contributor or Care Plan Service.
	// The HTTP client handles acquiring the required authentication credentials (e.g. OAuth2 access tokens).
	HttpClient() *http.Client
	// CsdDirectory returns the directory service for finding endpoints and organizations through Care Service Discovery (IHE-CSD).
	CsdDirectory() csd.Directory
	// Identities returns the identities of the local tenant (e.g., a care organization).
	Identities(ctx context.Context) ([]fhir.Identifier, error)
}
