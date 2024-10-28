//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/pubsub"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const basePath = "/cpc"
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
	carePlanServiceClient := fhirclient.New(cpsURL, httpClient, coolfhir.Config())
	result := &Service{
		config:                config,
		orcaPublicURL:         orcaPublicURL,
		carePlanServiceURL:    cpsURL,
		carePlanServiceClient: carePlanServiceClient,
		SessionManager:        sessionManager,
		scpHttpClient:         httpClient,
		profile:               profile,
		frontendUrl:           config.FrontendConfig.URL,
		fhirURL:               fhirURL,
		transport:             localFHIRStoreTransport,
		workflows:             taskengine.DefaultWorkflows(),
		questionnaireLoader:   taskengine.EmbeddedQuestionnaireLoader{},
	}
	pubsub.DefaultSubscribers.FhirSubscriptionNotify = result.handleNotification
	return result, nil
}

type Service struct {
	config                Config
	profile               profile.Provider
	orcaPublicURL         *url.URL
	SessionManager        *user.SessionManager
	frontendUrl           string
	carePlanServiceURL    *url.URL
	carePlanServiceClient fhirclient.Client
	// scpHttpClient is used to call remote Care Plan Contributors or Care Plan Service, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	scpHttpClient *http.Client
	fhirURL       *url.URL
	// transport is used to call the local FHIR store, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	transport           http.RoundTripper
	workflows           taskengine.Workflows
	questionnaireLoader taskengine.QuestionnaireLoader
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	baseURL := s.orcaPublicURL.JoinPath(basePath)
	s.profile.RegisterHTTPHandlers(basePath, baseURL, mux)
	//
	// Authorized endpoints
	//
	mux.HandleFunc("POST "+basePath+"/fhir/notify", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		var notification coolfhir.SubscriptionNotification
		if err := json.NewDecoder(request.Body).Decode(&notification); err != nil {
			log.Error().Err(err).Msg("Failed to decode notification")
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/Notify"), writer)
			return
		}
		if err := s.handleNotification(request.Context(), &notification); err != nil {
			log.Error().Err(err).Msg("Failed to handle notification")
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/Notify"), writer)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	mux.HandleFunc(fmt.Sprintf("GET %s/fhir/{rest...}", basePath), s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleProxyToFHIR(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	}))
	//
	// FE/Session Authorized Endpoints
	//
	mux.HandleFunc("GET "+basePath+"/context", s.withSession(s.handleGetContext))
	mux.HandleFunc(basePath+"/ehr/fhir/{rest...}", s.withSession(s.handleProxyToEPD))
	carePlanServiceProxy := coolfhir.NewProxy(log.Logger, s.carePlanServiceURL, basePath+"/cps/fhir", s.scpHttpClient.Transport)
	mux.HandleFunc(basePath+"/cps/fhir/{rest...}", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
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
	// Verify that the u.Path refers to a careplan
	if !strings.HasPrefix(u.Path, "/cps/CarePlan/") {
		return errors.New("specified SCP context header does not refer to a CarePlan")
	}

	var bundle fhir.Bundle
	// TODO: Discuss changes to this validation with team
	// Use extract CarePlan ID to be used for our query that will get the CarePlan and CareTeam in a bundle
	carePlanId := strings.TrimPrefix(strings.TrimPrefix(u.Path, "/cps/CarePlan/"), s.carePlanServiceURL.String())
	err = s.carePlanServiceClient.Read("CarePlan", &bundle, fhirclient.QueryParam("_id", carePlanId), fhirclient.QueryParam("_include", "CarePlan:care-team"))
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

	// get the CareTeamParticipant, then check that it is active
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeams, principal.Organization.Identifier)
	if participant == nil {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}
	isValid, err := coolfhir.ValidateCareTeamParticipantPeriod(*participant, time.Now())
	if err != nil {
		return err
	}

	if !isValid {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}
	fhirProxy := coolfhir.NewProxy(log.Logger, s.fhirURL, basePath+"/fhir", s.transport)
	fhirProxy.ServeHTTP(writer, request)
	return nil
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

func (s Service) handleNotification(ctx context.Context, resource any) error {
	notification, ok := resource.(*coolfhir.SubscriptionNotification)
	if !ok {
		return &coolfhir.ErrorWithCode{
			Message:    "failed to cast resource to notification",
			StatusCode: http.StatusBadRequest,
		}
	}
	if notification == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "notification is nil",
			StatusCode: http.StatusInternalServerError,
		}
	}

	focusReference, err := notification.GetFocus()
	if err != nil {
		return err
	}
	if focusReference == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Notification focus not found",
			StatusCode: http.StatusUnprocessableEntity,
		}
	}

	if focusReference.Type == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Notification focus type is nil",
			StatusCode: http.StatusUnprocessableEntity,
		}
	}

	log.Info().Msgf("Received notification: Reference %s, Type: %s", *focusReference.Reference, *focusReference.Type)

	switch *focusReference.Type {
	case "Task":
		// Get task
		var task fhir.Task
		err = s.carePlanServiceClient.Read(*focusReference.Reference, &task)
		if err != nil {
			return err
		}

		// TODO: How to differentiate between create and update? (Currently we only use Create in CPS. There is code for Update but nothing calls it)
		err = s.handleTaskFillerCreate(ctx, &task)
		if err != nil {
			return err
		}
	default:
		log.Info().Msgf("Received notification of type %s is not yet supported", *focusReference.Type)
	}

	return nil
}
