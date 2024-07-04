package careplanservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
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
		// TODO: Authorize request here
		// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
		log.Info().Msg("Creating Task")
		// Resolve CarePlan for Task
		var task map[string]interface{}
		if err := s.readRequest(request, &task); err != nil {
			// TODO: proper OperationOutcome
			http.Error(writer, "Failed to read Task from HTTP request: "+err.Error(), http.StatusBadRequest)
			return
		}

		var taskBasedOn []fhir.Reference
		if err := convertInto(task["basedOn"], &taskBasedOn); err != nil {
			// TODO: proper OperationOutcome
			http.Error(writer, "Failed to convert Task.basedOn: "+err.Error(), http.StatusBadRequest)
			return
		} else if len(taskBasedOn) != 1 {
			// TODO: proper OperationOutcome
			http.Error(writer, "Task.basedOn must have exactly one reference", http.StatusBadRequest)
			return
		} else if taskBasedOn[0].Type == nil || *taskBasedOn[0].Type != "CarePlan" || taskBasedOn[0].Reference == nil {
			// TODO: proper OperationOutcome
			http.Error(writer, "Task.basedOn must reference a CarePlan", http.StatusBadRequest)
			return
		}
		// TODO: Manage time-outs properly
		var carePlan fhir.CarePlan
		if err := s.fhirClient.Read(*taskBasedOn[0].Reference, &carePlan); err != nil {
			// TODO: proper OperationOutcome
			http.Error(writer, "Failed to read CarePlan: "+err.Error(), http.StatusBadGateway)
			return
		}
		// Add Task to CarePlan.activities
		taskFullURL := "urn:uuid:" + uuid.NewString()
		carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
			Reference: &fhir.Reference{
				Reference: to.Ptr(taskFullURL),
				Type:      to.Ptr("Task"),
			},
		})
		carePlanData, _ := json.Marshal(carePlan)
		// Create Task and update CarePlan.activities in a single transaction
		// TODO: Only if not updated
		taskData, _ := json.Marshal(task)
		bundle := fhir.Bundle{
			Type: fhir.BundleTypeTransaction,
			Entry: []fhir.BundleEntry{
				// Create Task
				{
					FullUrl:  to.Ptr(taskFullURL),
					Resource: taskData,
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbPOST,
						Url:    "Task",
					},
				},
				// Update CarePlan
				{
					Resource: carePlanData,
					Request: &fhir.BundleEntryRequest{
						Method: fhir.HTTPVerbPUT,
						Url:    *taskBasedOn[0].Reference,
					},
				},
			},
		}
		if err := s.fhirClient.Create(bundle, &bundle, fhirclient.AtPath("/")); err != nil {
			// TODO: proper OperationOutcome
			http.Error(writer, "Failed to create Task and update CarePlan: "+err.Error(), http.StatusBadGateway)
			return
		}

		// Return Task
		for _, entry := range bundle.Entry {
			if entry.Response != nil && entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/") {
				if err := s.fhirClient.Read(*entry.Response.Location, &task); err != nil {
					http.Error(writer, "Failed to read created Task from FHIR server: "+err.Error(), http.StatusInternalServerError)
					return
				}
				// TODO: Get headers from FHIRClient.Read(), and pass them onto the response
				writer.WriteHeader(http.StatusCreated)
				writer.Header().Set("Content-Type", "application/json+fhir")
				_ = json.NewEncoder(writer).Encode(task)
				return
			}
		}

		// TODO: proper OperationOutcome
		http.Error(writer, "Could not find Task in FHIR Bundle", http.StatusInternalServerError)
	})
	mux.HandleFunc("/cps/*", func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
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
	return l.next.RoundTrip(request)
}
