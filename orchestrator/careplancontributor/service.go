//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
)

const basePath = "/contrib"
const LandingURL = basePath + "/"

// The care plan header key may be provided as X-SCP-Context but will be changed due to the Go http client canonicalization
const carePlanURLHeaderKey = "X-Scp-Context"

const CarePlanServiceOAuth2Scope = "careplanservice"

func New(
	config Config,
	profile profile.Provider,
	orcaPublicURL *url.URL,
	sessionManager *user.SessionManager) (*Service, error) {

	fhirURL, _ := url.Parse(config.FHIR.BaseURL)
	cpsURL, _ := url.Parse(config.CarePlanService.URL)

	fhirClientConfig := coolfhir.Config()
	localFHIRStoreTransport, _, err := coolfhir.NewAuthRoundTripper(config.FHIR, fhirClientConfig)
	if err != nil {
		return nil, err
	}
	httpClient := profile.HttpClient()
	return &Service{
		config:             config,
		orcaPublicURL:      orcaPublicURL,
		carePlanServiceURL: cpsURL,
		SessionManager:     sessionManager,
		scpHttpClient:      httpClient,
		profile:            profile,
		frontendUrl:        config.FrontendConfig.URL,
		fhirURL:            fhirURL,
		transport:          localFHIRStoreTransport,
	}, nil
}

type Service struct {
	config             Config
	profile            profile.Provider
	orcaPublicURL      *url.URL
	SessionManager     *user.SessionManager
	frontendUrl        string
	carePlanServiceURL *url.URL
	// scpHttpClient is used to call remote Care Plan Contributors or Care Plan Service, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	scpHttpClient *http.Client
	fhirURL       *url.URL
	// transport is used to call the local FHIR store, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	transport http.RoundTripper
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	baseURL := s.orcaPublicURL.JoinPath(basePath)
	s.profile.RegisterHTTPHandlers(basePath, baseURL, mux)
	//
	// Authorized endpoints
	//
	// TODO: Modify this and other URLs to /cpc/* in future change
	mux.HandleFunc(fmt.Sprintf("GET %s/fhir/*", basePath), s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleProxyToFHIR(writer, request)
		if err != nil {
			//http.Error(writer, err.Error(), http.StatusInternalServerError)
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("%s/fhir/*", basePath), writer)
			return
		}
	}))
	//
	// FE/Session Authorized Endpoints
	//
	mux.HandleFunc("GET "+basePath+"/context", s.withSession(s.handleGetContext))
	mux.HandleFunc("GET "+basePath+"/patient", s.withSession(s.handleGetPatient))
	mux.HandleFunc("GET "+basePath+"/practitioner", s.withSession(s.handleGetPractitioner))
	mux.HandleFunc("GET "+basePath+"/serviceRequest", s.withSession(s.handleGetServiceRequest))
	mux.HandleFunc(basePath+"/ehr/fhir/*", s.withSession(s.handleProxyToEPD))
	carePlanServiceProxy := coolfhir.NewProxy(log.Logger, s.carePlanServiceURL, basePath+"/cps/fhir", s.scpHttpClient.Transport)
	mux.HandleFunc(basePath+"/cps/fhir/*", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
		carePlanServiceProxy.ServeHTTP(writer, request)
	}))
	mux.HandleFunc(basePath+"/", func(response http.ResponseWriter, request *http.Request) {
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
	proxy := coolfhir.NewProxy(log.Logger, clientFactory.BaseURL, basePath+"/ehr/fhir", clientFactory.Client)
	proxy.ServeHTTP(writer, request)
}

func (s Service) handleProxyToFHIR(writer http.ResponseWriter, request *http.Request) error {
	// Authorize requester before proxying FHIR request
	// Data holder must verify that the requester is part of the CareTeam by checking the URA
	// Validate by retrieving the CarePlan from CPS, use URA in provided token to validate against CareTeam
	// CarePlan should be provided in X-Scp-Context header
	carePlanURLValue := request.Header[carePlanURLHeaderKey]
	if len(carePlanURLValue) == 0 {
		return errors.New(fmt.Sprintf("%s header value must be set", carePlanURLHeaderKey))
	}
	if len(carePlanURLValue) != 1 {
		return errors.New(fmt.Sprintf("%s header must only contain one value", carePlanURLHeaderKey))
	}
	carePlanURL := carePlanURLValue[0]
	if carePlanURL == "" {
		return errors.New(fmt.Sprintf("%s header value must be set", carePlanURLHeaderKey))
	}
	if !strings.HasPrefix(carePlanURL, s.carePlanServiceURL.String()) {
		return errors.New("invalid CarePlan URL in header")
	}
	u, err := url.Parse(carePlanURL)
	if err != nil {
		return err
	}

	carePlanServiceClient := fhirclient.New(s.carePlanServiceURL, s.scpHttpClient, coolfhir.Config())
	var bundle fhir.Bundle
	// Use extract CarePlan ID to be used for our query that will get the CarePlan and CareTeam in a bundle
	carePlanId := strings.TrimPrefix(strings.TrimPrefix(u.Path, "/fhir/CarePlan/"), s.carePlanServiceURL.String())
	newUrl := fmt.Sprintf("CarePlan?_id=%s&_include=CarePlan:care-team", carePlanId)
	err = carePlanServiceClient.Read(newUrl, &bundle)
	if err != nil {
		return err
	}

	if len(bundle.Entry) == 0 {
		return coolfhir.NewErrorWithCode("CarePlan not found", http.StatusNotFound)
	}

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return err
	}
	if len(careTeams) == 0 {
		return coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
	}

	// Validate CareTeam participants against requester
	principal, err := auth.PrincipalFromContext(request.Context())
	if err != nil {
		return err
	}
	isValidRequester := false
	for _, careTeam := range careTeams {
		for _, participant := range careTeam.Participant {
			for _, identifier := range principal.Organization.Identifier {
				if coolfhir.IdentifierEquals(participant.Member.Identifier, &identifier) {
					// Member must have start date, this date must be in the past, and if there is an end date then it must be in the future
					err = coolfhir.ValidateCareTeamParticipantPeriod(participant)
					if err != nil {
						return err
					}
					isValidRequester = true
					break
				}
			}
		}
	}

	if !isValidRequester {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}
	fhirProxy := coolfhir.NewProxy(log.Logger, s.fhirURL, basePath+"/fhir", s.transport)
	fhirProxy.ServeHTTP(writer, request)
	return nil
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

func (s Service) withSessionOrBearerToken(next func(response http.ResponseWriter, request *http.Request)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		// TODO: Change this to something more sophisticated (OpenID Connect?)
		if (s.config.StaticBearerToken != "" && request.Header.Get("Authorization") == "Bearer "+s.config.StaticBearerToken) ||
			s.SessionManager.Get(request) != nil {
			next(response, request)
			return
		}
		http.Error(response, "no session found", http.StatusUnauthorized)
	}
}
