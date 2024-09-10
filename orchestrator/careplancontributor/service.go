//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/SanteonNL/orca/orchestrator/addressing"
	"github.com/SanteonNL/orca/orchestrator/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/user"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

const LandingURL = "/contrib/"
const CarePlanServiceOAuth2Scope = "careplanservice"

func New(config Config, sessionManager *user.SessionManager, carePlanServiceHttpClient *http.Client, didResolver addressing.StaticDIDResolver) (*Service, error) {
	fhirURL, _ := url.Parse(config.FHIR.BaseURL)
	cpsURL, _ := url.Parse(config.CarePlanService.URL)
	carePlanServiceClient := fhirclient.New(cpsURL, carePlanServiceHttpClient, coolfhir.Config())
	var transport http.RoundTripper
	switch config.FHIR.Auth.Type {
	case "azure-managedidentity":
		credential, err := azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("unable to get credential for Azure FHIR API client: %w", err)
		}
		httpClient := coolfhir.NewAzureHTTPClient(credential, coolfhir.DefaultAzureScope(fhirURL))
		transport = httpClient.Transport
	case "":
		transport = http.DefaultTransport
	default:
		return nil, fmt.Errorf("invalid FHIR authentication type: %s", config.FHIR.Auth.Type)
	}
	return &Service{
		carePlanServiceURL:        cpsURL,
		SessionManager:            sessionManager,
		carePlanService:           carePlanServiceClient,
		carePlanServiceHttpClient: carePlanServiceHttpClient,
		frontendUrl:               config.FrontendConfig.URL,
		fhirURL:                   fhirURL,
		transport:                 transport,
	}, nil
}

type Service struct {
	SessionManager            *user.SessionManager
	frontendUrl               string
	carePlanService           fhirclient.Client
	carePlanServiceURL        *url.URL
	carePlanServiceHttpClient *http.Client
	fhirURL                   *url.URL
	transport                 http.RoundTripper
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /contrib/context", s.withSession(s.handleGetContext))
	mux.HandleFunc("GET /contrib/patient", s.withSession(s.handleGetPatient))
	mux.HandleFunc("GET /contrib/practitioner", s.withSession(s.handleGetPractitioner))
	mux.HandleFunc("GET /contrib/serviceRequest", s.withSession(s.handleGetServiceRequest))
	mux.HandleFunc("/contrib/ehr/fhir/*", s.withSession(s.handleProxyToEPD))
	carePlanServiceProxy := coolfhir.NewProxy(log.Logger, s.carePlanServiceURL, "/contrib/cps/fhir", s.carePlanServiceHttpClient.Transport)
	mux.HandleFunc("/contrib/cps/fhir/*", s.withSession(func(writer http.ResponseWriter, request *http.Request, _ *user.SessionData) {
		carePlanServiceProxy.ServeHTTP(writer, request)
	}))
	fhirProxy := coolfhir.NewProxy(log.Logger, s.fhirURL, "/contrib/fhir", s.transport)
	// TODO: Determine auth. CareTeam can be sent in header but this is not ideal, preferably put it in the access token
	// TODO: Modify this and other URLs to /cpc/* in future change
	mux.HandleFunc("/contrib/fhir/*", s.withSession(func(writer http.ResponseWriter, request *http.Request, _ *user.SessionData) {
		// TODO: Is this correct?
		fhirProxy.ServeHTTP(writer, request)
	}))
	mux.HandleFunc("/contrib/", func(response http.ResponseWriter, request *http.Request) {
		log.Info().Msgf("Redirecting to %s", s.frontendUrl)
		http.Redirect(response, request, s.frontendUrl, http.StatusFound)
	})
}

// withSession is a middleware that retrieves the session for the given request.
// It then calls the given handler function and provides the session.
// If there's no active session, it returns a 401 Unauthorized response.
func (s Service) withSession(next func(response http.ResponseWriter, request *http.Request, session *user.SessionData)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		session := s.SessionManager.Get(request)
		if session == nil {
			http.Error(response, "no session found", http.StatusUnauthorized)
			return
		}
		next(response, request, session)
	}
}

func (s Service) handleProxyToEPD(writer http.ResponseWriter, request *http.Request, session *user.SessionData) {
	clientFactory := clients.Factories[session.FHIRLauncher](session.Values)
	proxy := coolfhir.NewProxy(log.Logger, clientFactory.BaseURL, "/contrib/ehr/fhir", clientFactory.Client)
	proxy.ServeHTTP(writer, request)
}

func (s Service) handleProxyToFHIR(writer http.ResponseWriter, request *http.Request, session *user.SessionData) {
	clientFactory := clients.Factories[session.FHIRLauncher](session.Values)
	proxy := coolfhir.NewProxy(log.Logger, clientFactory.BaseURL, "/contrib/fhir", clientFactory.Client)
	proxy.ServeHTTP(writer, request)
}

// handleGetPatient is the REST API handler that returns the FHIR Patient.
func (s Service) handleGetPatient(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())

	var patient fhir.Patient
	if err := fhirClient.Read(session.Values["patient"], &patient); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(patient)
	_, _ = response.Write(data)
}

// handleGetPractitioner is the REST API handler that returns the FHIR Practitioner.
func (s Service) handleGetPractitioner(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())
	var practitioner fhir.Practitioner
	if err := fhirClient.Read(session.Values["practitioner"], &practitioner); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(practitioner)
	_, _ = response.Write(data)
}

// handleGetServiceRequest is the REST API handler that returns the FHIR ServiceRequest.
func (s Service) handleGetServiceRequest(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())
	var serviceRequest fhir.ServiceRequest
	if err := fhirClient.Read(session.Values["serviceRequest"], &serviceRequest); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(serviceRequest)
	_, _ = response.Write(data)
}

func (s Service) readServiceRequest(localFHIR fhirclient.Client, serviceRequestRef string) (*fhir.ServiceRequest, error) {
	// TODO: Make this complete, and test this
	var serviceRequest fhir.ServiceRequest
	if err := localFHIR.Read(serviceRequestRef, &serviceRequest); err != nil {
		return nil, err
	}

	serviceRequest.ReasonReference = nil
	var serviceRequestReasons []map[string]interface{}
	for i, reasonReference := range serviceRequest.ReasonReference {
		// TODO: ReasonReference should probably be an ID instead of logical identifier?
		if reasonReference.Identifier == nil || reasonReference.Identifier.Value == nil {
			return nil, fmt.Errorf("expected ServiceRequest.reasonReference[%d].identifier.value to be set", i)
		}
		results := make([]map[string]interface{}, 0)
		// TODO: Just taking first isn't right, fix with technical IDs instead of logical identifiers
		if err := localFHIR.Read(*reasonReference.Type+"/?identifier="+*reasonReference.Identifier.Value, &results); err != nil {
			return nil, err
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("could not resolve ServiceRequest.reasonReference[%d].identifier", i)
		}
		reason := results[0]
		ref := fmt.Sprintf("#servicerequest-reason-%d", i+1)
		reason["id"] = ref
		serviceRequestReasons = append(serviceRequestReasons, results[0])
		serviceRequest.ReasonReference = append(serviceRequest.ReasonReference, fhir.Reference{
			Type:      to.Ptr(*reasonReference.Type),
			Reference: to.Ptr(ref),
		})
	}
	return &serviceRequest, nil
}

func (s Service) handleGetContext(response http.ResponseWriter, _ *http.Request, session *user.SessionData) {
	contextData := struct {
		Patient        string `json:"patient"`
		ServiceRequest string `json:"serviceRequest"`
		Practitioner   string `json:"practitioner"`
	}{
		Patient:        session.Values["patient"],
		ServiceRequest: session.Values["serviceRequest"],
		Practitioner:   session.Values["practitioner"],
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(contextData)
}
