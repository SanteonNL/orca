package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"io"
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
	var fhirClient fhirclient.Client
	fhirClientConfig := coolfhir.Config()
	switch config.FHIR.Auth.Type {
	case "azure-managedidentity":
		credential, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
		}
		httpClient := coolfhir.NewAzureHTTPClient(credential, coolfhir.DefaultAzureScope(fhirURL))
		transport = httpClient.Transport
		fhirClient = fhirclient.New(fhirURL, httpClient, fhirClientConfig)
	case "":
		transport = http.DefaultTransport
		fhirClient = fhirclient.New(fhirURL, http.DefaultClient, fhirClientConfig)
	default:
		return nil, fmt.Errorf("invalid FHIR authentication type: %s", config.FHIR.Auth.Type)
	}
	return &Service{
		fhirURL:         fhirURL,
		didResolver:     didResolver,
		transport:       transport,
		fhirClient:      fhirClient,
		maxReadBodySize: fhirClientConfig.MaxResponseSize,
	}, nil
}

type Service struct {
	didResolver     addressing.DIDResolver
	fhirURL         *url.URL
	transport       http.RoundTripper
	fhirClient      fhirclient.Client
	maxReadBodySize int
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
		err := s.handleCreateTask(writer, request)
		if err != nil {
			// TODO: proper OperationOutcome
			log.Info().Msgf("CarePlanService/CreateTask failed: %v", err)
			http.Error(writer, "Create Task at CarePlanService failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	})
	mux.HandleFunc("POST /cps/CarePlan", func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleCreateCarePlan(writer, request)
		if err != nil {
			// TODO: proper OperationOutcome
			log.Info().Msgf("CarePlanService/CarePlan failed: %v", err)
			http.Error(writer, "Create CarePlan at CarePlanService failed: "+err.Error(), http.StatusBadRequest)
			return
		}
	})
	mux.HandleFunc("/cps/*", func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.Method, request.URL.String())
		proxy.ServeHTTP(writer, request)
	})
}

func (s Service) readRequest(httpRequest *http.Request, target interface{}) error {
	data, err := io.ReadAll(io.LimitReader(httpRequest.Body, int64(s.maxReadBodySize+1)))
	if err != nil {
		return err
	}
	if len(data) > s.maxReadBodySize {
		return fmt.Errorf("FHIR request body exceeds max. safety limit of %d bytes (%s %s)", s.maxReadBodySize, httpRequest.Method, httpRequest.URL.String())
	}
	return json.Unmarshal(data, target)
}

// convertInto converts the src object into the target object,
// by marshalling src to JSON and then unmarshalling it into target.
func convertInto(src interface{}, target interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

var _ http.RoundTripper = &loggingRoundTripper{}

type loggingRoundTripper struct {
	next http.RoundTripper
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	log.Info().Msgf("Proxying FHIR request: %s %s", request.Method, request.URL.String())
	if log.Debug().Enabled() {
		var headers []string
		for key, values := range request.Header {
			headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
		}
		log.Debug().Msgf("Proxy request headers: %s", strings.Join(headers, ", "))
	}
	response, err := l.next.RoundTrip(request)
	if err != nil {
		log.Warn().Err(err).Msgf("Proxied FHIR request failed (url=%s)", request.URL.String())
	} else {
		if log.Debug().Enabled() {
			log.Debug().Msgf("Proxied FHIR request response: %s", response.Status)
			var headers []string
			for key, values := range response.Header {
				headers = append(headers, fmt.Sprintf("(%s: %s)", key, strings.Join(values, ", ")))
			}
			log.Debug().Msgf("Proxy response headers: %s", strings.Join(headers, ", "))
		}
	}
	return response, err
}
