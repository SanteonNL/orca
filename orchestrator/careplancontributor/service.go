//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

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
)

const basePath = "/cpc"
const LandingURL = basePath + "/"

// The care plan header key may be provided as X-SCP-Context but will be changed due to the Go http client canonicalization
const carePlanURLHeaderKey = "X-Scp-Context"

const CarePlanServiceOAuth2Scope = "careplanservice"

type ScpValidationResult struct {
	isMember  bool
	carePlan  *fhir.CarePlan
	careTeams *[]fhir.CareTeam
}

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
	result := &Service{
		config:                  config,
		orcaPublicURL:           orcaPublicURL,
		localCarePlanServiceUrl: cpsURL,
		SessionManager:          sessionManager,
		scpHttpClient:           httpClient,
		profile:                 profile,
		frontendUrl:             config.FrontendConfig.URL,
		fhirURL:                 fhirURL,
		transport:               localFHIRStoreTransport,
		workflows:               taskengine.DefaultWorkflows(),
		questionnaireLoader:     taskengine.EmbeddedQuestionnaireLoader{},
		cpsClientFactory: func(baseURL *url.URL) fhirclient.Client {
			return fhirclient.New(baseURL, httpClient, coolfhir.Config())
		},
		healthdataviewEndpointEnabled: config.HealthDataViewEndpointEnabled,
	}
	pubsub.DefaultSubscribers.FhirSubscriptionNotify = result.handleNotification
	return result, nil
}

type Service struct {
	config         Config
	profile        profile.Provider
	orcaPublicURL  *url.URL
	SessionManager *user.SessionManager
	frontendUrl    string
	// localCarePlanServiceUrl is the URL of the local Care Plan Service, used to create new CarePlans.
	localCarePlanServiceUrl *url.URL
	// scpHttpClient is used to call remote Care Plan Contributors or Care Plan Service, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	scpHttpClient *http.Client
	// cpsClientFactory is a factory function that creates a new FHIR client for any CarePlanService.
	cpsClientFactory func(baseURL *url.URL) fhirclient.Client
	fhirURL          *url.URL
	// transport is used to call the local FHIR store, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	transport                     http.RoundTripper
	workflows                     taskengine.Workflows
	questionnaireLoader           taskengine.QuestionnaireLoader
	healthdataviewEndpointEnabled bool
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
	// The aggregate endpoint is used to proxy requests to all CarePlanContributors in the CarePlan. It is used by the HealthDataView to aggregate data from all CarePlanContributors.
	mux.HandleFunc(fmt.Sprintf("GET %s/aggregate/fhir/{rest...}", basePath), s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if !s.healthdataviewEndpointEnabled {
			coolfhir.WriteOperationOutcomeFromError(&coolfhir.ErrorWithCode{
				Message:    "health data view proxy endpoint is disabled",
				StatusCode: http.StatusMethodNotAllowed,
			}, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}

		err := s.proxyToAllCareTeamMembers(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	}))
	// BgZ-specific endpoints. Similar to `GET %s/fhir/{rest...}` but the proxied request requires to have a workflow-specific Access Token that must be created by orca
	mux.HandleFunc(fmt.Sprintf("GET %s/bgz/fhir/{rest...}", basePath), s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if !s.healthdataviewEndpointEnabled {
			coolfhir.WriteOperationOutcomeFromError(&coolfhir.ErrorWithCode{
				Message:    "health data view proxy endpoint is disabled",
				StatusCode: http.StatusMethodNotAllowed,
			}, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}

		err := s.handleProxyBgzData(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	}))
	mux.HandleFunc(fmt.Sprintf("GET %s/fhir/{rest...}", basePath), s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if !s.healthdataviewEndpointEnabled {
			coolfhir.WriteOperationOutcomeFromError(&coolfhir.ErrorWithCode{
				Message:    "health data view proxy endpoint is disabled",
				StatusCode: http.StatusMethodNotAllowed,
			}, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}

		err := s.handleProxyExternalRequestToEHR(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	}))
	//
	// FE/Session Authorized Endpoints
	//
	mux.HandleFunc("GET "+basePath+"/context", s.withSession(s.handleGetContext))
	mux.HandleFunc(basePath+"/ehr/fhir/{rest...}", s.withSession(s.handleProxyAppRequestToEHR))
	carePlanServiceProxy := coolfhir.NewProxy(log.Logger, s.localCarePlanServiceUrl, basePath+"/cps/fhir", s.scpHttpClient.Transport)
	mux.HandleFunc(basePath+"/cps/fhir/{rest...}", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
		carePlanServiceProxy.ServeHTTP(writer, request)
	}))
	mux.HandleFunc(basePath+"/", func(response http.ResponseWriter, request *http.Request) {
		log.Info().Msgf("Redirecting to %s", s.frontendUrl)
		http.Redirect(response, request, s.frontendUrl, http.StatusFound)
	})

	// Logout endpoint
	mux.HandleFunc("/logout", s.withSession(func(writer http.ResponseWriter, request *http.Request, session *user.SessionData) {
		s.SessionManager.Destroy(writer, request)
		// If there is a 'Referer' value in the header, redirect to that URL
		if referer := request.Header.Get("Referer"); referer != "" {
			http.Redirect(writer, request, referer, http.StatusFound)
		} else {
			// This redirection will be handled by middleware in the frontend
			http.Redirect(writer, request, s.frontendUrl, http.StatusOK)
		}
	}))
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

// handleProxyAppRequestToEHR handles a request from the CPC application (e.g. Frontend), forwarding it to the local EHR's FHIR API.
func (s Service) handleProxyAppRequestToEHR(writer http.ResponseWriter, request *http.Request, session *user.SessionData) {
	clientFactory := clients.Factories[session.FHIRLauncher](session.StringValues)
	proxy := coolfhir.NewProxy(log.Logger, clientFactory.BaseURL, basePath+"/ehr/fhir", clientFactory.Client)

	resourcePath := request.PathValue("rest")
	// If the requested resource is cached in the session, directly return it. This is used to support resources that are required (e.g. by Frontend), but not provided by the EHR.
	// E.g., ChipSoft HiX doesn't provide ServiceRequest and Practitioner as FHIR resources, so whatever there is, is converted to FHIR and cached in the session.
	if resource, exists := session.OtherValues[resourcePath]; exists {
		coolfhir.SendResponse(writer, http.StatusOK, resource)
	} else {
		proxy.ServeHTTP(writer, request)
	}
}

// handleProxyExternalRequestToEHR handles a request from an external SCP-node (e.g. CarePlanContributor), forwarding it to the local EHR's FHIR API.
// This is typically used by remote parties to retrieve patient data from the local EHR.
func (s Service) handleProxyExternalRequestToEHR(writer http.ResponseWriter, request *http.Request) error {

	result, err := s.authorizeScpMember(request)

	if err != nil {
		log.Error().Err(err).Msg("Failed to authorize SCP member")
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusBadRequest)
	}

	if !result.isMember {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}

	fhirProxy := coolfhir.NewProxy(log.Logger, s.fhirURL, basePath+"/fhir", s.transport)
	fhirProxy.ServeHTTP(writer, request)
	return nil
}

// proxyToAllCareTeamMembers is a convenience façade method that can be used proxy the request to all CPC nodes localized from the Shared CarePlan.participants.
func (s *Service) proxyToAllCareTeamMembers(writer http.ResponseWriter, request *http.Request) error {
	result, err := s.authorizeScpMember(request)

	if err != nil {
		log.Error().Err(err).Msg("Failed to authorize SCP member")
		return coolfhir.BadRequestError(err)
	}

	if !result.isMember {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}

	//TODO: We currently have a 1-on-1 relation between the EHR and viewer. Should query all CarePlan.participants
	// carePlanURLValue := request.Header[carePlanURLHeaderKey]

	//TODO: This URL should come from localization based on each participants URA number - for now fixate it to the hospital url
	proxyBaseUrl, err := url.Parse(strings.Replace(s.config.CarePlanService.URL, "/cps", "", 1))

	if err != nil {
		return coolfhir.BadRequestError(err)
	}

	fhirProxy := coolfhir.NewProxy(log.Logger, proxyBaseUrl, basePath+"/bgz/fhir", s.transport)
	fhirProxy.ServeHTTP(writer, request)

	//TODO: Figure out how to combine result bundles
	return nil
}

// handleProxyBgzData handles a request from an external SCP-node (e.g. CarePlanContributor), forwarding it to the local EHR's FHIR API.
func (s Service) handleProxyBgzData(writer http.ResponseWriter, request *http.Request) error {

	result, err := s.authorizeScpMember(request)

	if err != nil {
		log.Error().Err(err).Msg("Failed to authorize SCP member")
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusBadRequest)
	}

	if !result.isMember {
		return coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}

	// The original access_token has been validated, however, we now need to replace the proxy with a BgZ-specific one. We need to proxy the request, but NOT return the BgZ-specific access_token to the requester.
	// originalAccessToken := request.Header.Get("Authorization")
	// bgzAccessToken := s.serv

	fhirProxy := coolfhir.NewProxy(log.Logger, s.fhirURL, basePath+"/fhir", s.transport)
	fhirProxy.ServeHTTP(writer, request)
	return nil
}

func (s Service) authorizeScpMember(request *http.Request) (*ScpValidationResult, error) {
	// Authorize requester before proxying FHIR request
	// Data holder must verify that the requester is part of the CareTeam by checking the URA
	// Validate by retrieving the CarePlan from CPS, use URA in provided token to validate against CareTeam
	// CarePlan should be provided in X-Scp-Context header
	carePlanURLValue := request.Header[carePlanURLHeaderKey]
	if len(carePlanURLValue) != 1 {
		return nil, fmt.Errorf("%s header must only contain one value", carePlanURLHeaderKey)
	}
	carePlanURL := carePlanURLValue[0]
	if carePlanURL == "" {
		return nil, fmt.Errorf("%s header value must be set", carePlanURLHeaderKey)
	}
	if !strings.HasPrefix(carePlanURL, s.localCarePlanServiceUrl.String()) {
		return nil, errors.New("invalid CarePlan URL in header")
	}
	u, err := url.Parse(carePlanURL)
	if err != nil {
		return nil, err
	}
	// Verify that the u.Path refers to a careplan
	if !strings.HasPrefix(u.Path, "/cps/CarePlan/") {
		return nil, errors.New("specified SCP context header does not refer to a CarePlan")
	}

	var bundle fhir.Bundle
	// TODO: Discuss changes to this validation with team
	// Use extract CarePlan ID to be used for our query that will get the CarePlan and CareTeam in a bundle
	carePlanId := strings.TrimPrefix(strings.TrimPrefix(u.Path, "/cps/CarePlan/"), s.localCarePlanServiceUrl.String())
	err = s.cpsClientFactory(s.localCarePlanServiceUrl).Read("CarePlan", &bundle, fhirclient.QueryParam("_id", carePlanId), fhirclient.QueryParam("_include", "CarePlan:care-team"))
	if err != nil {
		return nil, err
	}

	if len(bundle.Entry) == 0 {
		return nil, coolfhir.NewErrorWithCode("CarePlan not found", http.StatusNotFound)
	}

	var careTeams []fhir.CareTeam
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if err != nil {
		return nil, err
	}
	if len(careTeams) == 0 {
		return nil, coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
	}

	var carePlan fhir.CarePlan
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CarePlan"), &carePlan)

	// Validate CareTeam participants against requester
	principal, err := auth.PrincipalFromContext(request.Context())
	if err != nil {
		return nil, err
	}

	// get the CareTeamParticipant, then check that it is active
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeams, principal.Organization.Identifier)
	if participant == nil {
		return nil, coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}

	isMember, err := coolfhir.ValidateCareTeamParticipantPeriod(*participant, time.Now())
	if err != nil {
		return nil, err
	}

	return &ScpValidationResult{
		isMember:  isMember,
		carePlan:  &carePlan,
		careTeams: &careTeams,
	}, nil
}

func (s Service) handleGetContext(response http.ResponseWriter, _ *http.Request, session *user.SessionData) {
	contextData := struct {
		Patient        string `json:"patient"`
		ServiceRequest string `json:"serviceRequest"`
		Practitioner   string `json:"practitioner"`
	}{
		Patient:        session.StringValues["patient"],
		ServiceRequest: session.StringValues["serviceRequest"],
		Practitioner:   session.StringValues["practitioner"],
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

	if focusReference.Reference == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Notification focus reference is nil",
			StatusCode: http.StatusUnprocessableEntity,
		}
	}
	resourceUrl := *focusReference.Reference
	if !strings.HasPrefix(strings.ToLower(resourceUrl), "http:") && !strings.HasPrefix(strings.ToLower(resourceUrl), "https:") {
		return &coolfhir.ErrorWithCode{
			Message:    "Notification focus.reference is not an absolute URL",
			StatusCode: http.StatusUnprocessableEntity,
		}
	}
	// TODO: for now, we assume the resource URL is always in the form of <FHIR base url>/<resource type>/<resource id>
	//       Then, we can deduce the FHIR base URL from the resource URL
	resourceUrlParts := strings.Split(resourceUrl, "/")
	resourceUrlParts = resourceUrlParts[:len(resourceUrlParts)-2]
	resourceBaseUrl := strings.Join(resourceUrlParts, "/")
	parsedResourceBaseUrl, err := url.Parse(resourceBaseUrl)
	if err != nil {
		return err
	}

	switch *focusReference.Type {
	case "Task":
		fhirClient := s.cpsClientFactory(parsedResourceBaseUrl)
		// Get task
		var task fhir.Task
		err = fhirClient.Read(*focusReference.Reference, &task)
		if err != nil {
			return err
		}

		// TODO: How to differentiate between create and update? (Currently we only use Create in CPS. There is code for Update but nothing calls it)
		err = s.handleTaskFillerCreateOrUpdate(ctx, fhirClient, &task)
		if err != nil {
			return err
		}
	default:
		log.Info().Msgf("Received notification of type %s is not yet supported", *focusReference.Type)
	}

	return nil
}
