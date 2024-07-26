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
	"net/url"
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
	proxy := coolfhir.NewProxy(log.Logger, s.fhirURL, "/cps", s.transport)
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
