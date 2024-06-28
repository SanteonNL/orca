package careplanservice

import (
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
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
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = s.fhirURL.JoinPath(strings.TrimPrefix(r.In.URL.Path, "/cps"))
		},
		Transport: s.httpClient.Transport,
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Warn().Err(err).Msgf("FHIR request failed (url=%s)", request.URL.String())
			http.Error(writer, "FHIR request failed: "+err.Error(), http.StatusBadGateway)
		},
	}
	mux.HandleFunc("/cps/*", func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		proxy.ServeHTTP(writer, request)
	})
}
