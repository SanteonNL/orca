package careplanservice

import (
	"errors"
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
	if config.FHIR.BaseURL == "" {
		return nil, errors.New("careplanservice.fhir.url is not configured")
	}
	fhirURL, err := url.Parse(config.FHIR.BaseURL)
	if err != nil {
		return nil, err
	}
	var transport http.RoundTripper
	switch config.FHIR.Auth.Type {
	case "azure-managedidentity":
		credential, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
		}
		transport = coolfhir.NewAzureHTTPClient(credential, coolfhir.DefaultAzureScope(fhirURL)).Transport
	case "":
		transport = http.DefaultTransport
	default:
		return nil, fmt.Errorf("invalid FHIR authentication type: %s", config.FHIR.Auth.Type)
	}
	return &Service{
		fhirURL:     fhirURL,
		didResolver: didResolver,
		transport:   transport,
	}, nil
}

type Service struct {
	didResolver addressing.DIDResolver
	fhirURL     *url.URL
	transport   http.RoundTripper
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL = s.fhirURL.JoinPath(strings.TrimPrefix(r.In.URL.Path, "/cps"))
			r.Out.Host = s.fhirURL.Host
		},
		Transport: loggingRoundTripper{
			next: s.transport,
		},
		ErrorHandler: func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Warn().Err(err).Msgf("FHIR request failed (url=%s)", request.URL.String())
			http.Error(writer, "FHIR request failed: "+err.Error(), http.StatusBadGateway)
		},
	}
	mux.HandleFunc("POST /cps/Task", func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		proxy.ServeHTTP(writer, request)
	})
	mux.HandleFunc("/cps/*", func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		proxy.ServeHTTP(writer, request)
	})
}

var _ http.RoundTripper = &loggingRoundTripper{}

type loggingRoundTripper struct {
	next http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	log.Info().Msgf("Proxying FHIR request: %s %s", request.Method, request.URL.String())
	return l.next.RoundTrip(request)
}
