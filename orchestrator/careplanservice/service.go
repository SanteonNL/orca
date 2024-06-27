package careplanservice

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func New(config Config, didResolver addressing.DIDResolver) (*Service, error) {
	fhirURL, err := url.Parse(config.FHIR.BaseURL)
	if err != nil {
		return nil, err
	}
	var httpClient *http.Client
	switch config.FHIR.Auth.Type {
	case "azure-managedidentity":
		credential, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
		}
		httpClient = coolfhir.NewAzureHTTPClient(credential, coolfhir.DefaultAzureScope(fhirURL))
	case "":
		httpClient = http.DefaultClient
	default:
		return nil, fmt.Errorf("invalid FHIR authentication type: %s", config.FHIR.Auth.Type)
	}
	return &Service{
		fhirURL:     fhirURL,
		didResolver: didResolver,
		httpClient:  httpClient,
	}, nil
}

type Service struct {
	didResolver addressing.DIDResolver
	fhirURL     *url.URL
	httpClient  *http.Client
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	proxy := httputil.NewSingleHostReverseProxy(s.fhirURL)
	proxy.Transport = s.httpClient.Transport
	mux.HandleFunc("/cps", proxy.ServeHTTP)
}
