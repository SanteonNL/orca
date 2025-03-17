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

	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/messaging"

	events "github.com/SanteonNL/orca/orchestrator/careplancontributor/event"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/webhook"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/ehr"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/pubsub"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const basePath = "/cpc"

// The care plan header key may be provided as X-SCP-Context but will be changed due to the Go http client canonicalization
const carePlanURLHeaderKey = "X-Scp-Context"

const CarePlanServiceOAuth2Scope = "careplanservice"

var fhirClientFactory = createFHIRClient

type ScpValidationResult struct {
	carePlan  *fhir.CarePlan
	careTeams *[]fhir.CareTeam
}

func New(
	config Config,
	profile profile.Provider,
	orcaPublicURL *url.URL,
	sessionManager *user.SessionManager,
	messageBroker messaging.Broker,
	ehrFhirProxy coolfhir.HttpProxy,
	localCarePlanServiceURL *url.URL) (*Service, error) {

	fhirURL, _ := url.Parse(config.FHIR.BaseURL)

	localFhirStoreTransport, _, err := coolfhir.NewAuthRoundTripper(config.FHIR, coolfhir.Config())
	if err != nil {
		return nil, err
	}

	// Initialize workflow provider, which is used to select FHIR Questionnaires by the Task Filler engine
	var workflowProvider taskengine.WorkflowProvider
	ctx := context.Background()
	if config.TaskFiller.QuestionnaireFHIR.BaseURL == "" {
		// Use embedded workflow provider
		memoryWorkflowProvider := &taskengine.MemoryWorkflowProvider{}
		for _, bundleUrl := range config.TaskFiller.QuestionnaireSyncURLs {
			log.Ctx(ctx).Info().Msgf("Loading Task Filler Questionnaires/HealthcareService resources from URL: %s", bundleUrl)
			if err := memoryWorkflowProvider.LoadBundle(ctx, bundleUrl); err != nil {
				return nil, fmt.Errorf("failed to load Task Filler Questionnaires/HealthcareService resources (url=%s): %w", bundleUrl, err)
			}
		}
		workflowProvider = memoryWorkflowProvider
	} else {
		// Use FHIR-based workflow provider
		_, questionnaireFhirClient, err := coolfhir.NewAuthRoundTripper(config.TaskFiller.QuestionnaireFHIR, coolfhir.Config())
		if err != nil {
			return nil, err
		}
		// Load Questionnaire-related resources for the Task Filler Engine from the configured URLs into the Questionnaire FHIR API
		go func(ctx context.Context, client fhirclient.Client) {
			if len(config.TaskFiller.QuestionnaireSyncURLs) > 0 {
				log.Ctx(ctx).Info().Msgf("Synchronizing Task Filler Questionnaires resources to local FHIR store from %d URLs", len(config.TaskFiller.QuestionnaireSyncURLs))
				for _, u := range config.TaskFiller.QuestionnaireSyncURLs {
					if err := coolfhir.ImportResources(ctx, questionnaireFhirClient, []string{"Questionnaire", "HealthcareService"}, u); err != nil {
						log.Ctx(ctx).Error().Err(err).Msgf("Failed to synchronize Task Filler Questionnaire resources (url=%s)", u)
					} else {
						log.Ctx(ctx).Debug().Msgf("Synchronized Task Filler Questionnaire resources (url=%s)", u)
					}
				}
			}
		}(ctx, questionnaireFhirClient)
		workflowProvider = taskengine.FhirApiWorkflowProvider{Client: questionnaireFhirClient}
	}

	// Register event handlers
	eventManager := events.NewInMemoryManager()
	for _, handler := range config.Events.WebHooks {
		err := eventManager.Subscribe(handler.ResourceType, handler.Name, webhook.NewEventHandler(handler.URL).Handle)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to event %s: %w", handler.Name, err)
		}
	}

	result := &Service{
		config:                        config,
		orcaPublicURL:                 orcaPublicURL,
		localCarePlanServiceUrl:       localCarePlanServiceURL,
		SessionManager:                sessionManager,
		profile:                       profile,
		frontendUrl:                   config.FrontendConfig.URL,
		fhirURL:                       fhirURL,
		ehrFhirProxy:                  ehrFhirProxy,
		transport:                     localFhirStoreTransport,
		workflows:                     workflowProvider,
		healthdataviewEndpointEnabled: config.HealthDataViewEndpointEnabled,
		eventManager:                  eventManager,
	}
	if config.TaskFiller.TaskAcceptedBundleTopic != "" {
		result.notifier = ehr.NewNotifier(messageBroker, config.TaskFiller.TaskAcceptedBundleTopic)
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
	fhirURL                 *url.URL
	ehrFhirProxy            coolfhir.HttpProxy
	// transport is used to call the local FHIR store, used to:
	// - proxy requests from the Frontend application (e.g. initiating task workflow)
	// - proxy requests from EHR (e.g. fetching remote FHIR data)
	transport                     http.RoundTripper
	workflows                     taskengine.WorkflowProvider
	healthdataviewEndpointEnabled bool
	notifier                      ehr.Notifier
	eventManager                  events.Manager
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	baseURL := s.orcaPublicURL.JoinPath(basePath)
	//
	// The section below defines endpoints specified by Shared Care Planning.
	// These are secured through the profile (e.g. Nuts access tokens)
	//
	handleBundle := func(httpRequest *http.Request) error {
		var notification fhir.Bundle
		if err := json.NewDecoder(httpRequest.Body).Decode(&notification); err != nil {
			return coolfhir.BadRequest("failed to decode bundle: %w", err)
		}
		if !coolfhir.IsSubscriptionNotification(&notification) {
			return coolfhir.BadRequest("bundle type not supported: " + notification.Type.String())
		}
		if err := s.handleNotification(httpRequest.Context(), (*coolfhir.SubscriptionNotification)(&notification)); err != nil {
			return err
		}
		return nil
	}
	// TODO: Remove /notify after all SCP nodes have been updated to use /fhir for delivering notifications
	mux.HandleFunc("POST "+basePath+"/fhir/notify", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if err := handleBundle(request); err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/CreateBundle", writer)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	mux.HandleFunc("POST "+basePath+"/fhir", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if err := handleBundle(request); err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/CreateBundle", writer)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	mux.HandleFunc("POST "+basePath+"/fhir/", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if err := handleBundle(request); err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/CreateBundle", writer)
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))

	// The code to GET or POST/_search are the same, so we can use the same handler for both
	proxyGetOrSearchHandler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		//TODO: Make this endpoint more secure, currently it is only allowed when strict mode is disabled
		if !s.healthdataviewEndpointEnabled || globals.StrictMode {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), &coolfhir.ErrorWithCode{
				Message:    "health data view proxy endpoint is disabled or strict mode is enabled",
				StatusCode: http.StatusMethodNotAllowed,
			}, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}

		err := s.handleProxyExternalRequestToEHR(writer, request)
		if err != nil {
			log.Ctx(request.Context()).Err(err).Msgf("FHIR request from external CPC to local EHR failed (url=%s)", request.URL.String())
			// If the error is a FHIR OperationOutcome, we should sanitize it before returning it
			var operationOutcomeErr fhirclient.OperationOutcomeError
			if errors.As(err, &operationOutcomeErr) {
				operationOutcomeErr.OperationOutcome = coolfhir.SanitizeOperationOutcome(operationOutcomeErr.OperationOutcome)
				err = operationOutcomeErr
			}
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	})
	mux.HandleFunc("GET "+basePath+"/fhir/{resourceType}/{id}", s.profile.Authenticator(baseURL, proxyGetOrSearchHandler))
	mux.HandleFunc("POST "+basePath+"/fhir/{resourceType}/_search", s.profile.Authenticator(baseURL, proxyGetOrSearchHandler))
	//
	// The section below defines endpoints used for integrating the local EHR with ORCA.
	// They are NOT specified by SCP. Authorization is specific to the local EHR.
	//
	// The aggregate endpoint is used to proxy requests to all CarePlanContributors in the CarePlan. It is used by the HealthDataView to aggregate data from all CarePlanContributors.
	mux.HandleFunc("POST "+basePath+"/aggregate/fhir/{resourceType}/_search", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
		log.Ctx(request.Context()).Debug().Msg("Handling aggregate _search FHIR API request")
		err := s.proxyToAllCareTeamMembers(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributer/%s %s", request.Method, request.URL.Path), writer)
			return
		}
	}))
	mux.HandleFunc("GET "+basePath+"/context", s.withSession(s.handleGetContext))
	mux.HandleFunc(basePath+"/ehr/fhir/{rest...}", s.withSession(s.handleProxyAppRequestToEHR))
	proxyBasePath := basePath + "/cps/fhir"

	mux.HandleFunc(basePath+"/cps/fhir/{rest...}", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
		if s.localCarePlanServiceUrl == nil || s.localCarePlanServiceUrl.String() == "" {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), coolfhir.BadRequest("This ORCA instance has no local CarePlanService, API can't be used."), fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
		// TODO: Since we should only call our own CPS, this should be a local call.
		//       If not, this should be optimized so that we don't do it every call
		_, httpClient, err := s.createFHIRClientForURL(request.Context(), s.localCarePlanServiceUrl)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
		carePlanServiceProxy := coolfhir.NewProxy("App->CPS FHIR proxy", s.localCarePlanServiceUrl,
			proxyBasePath, s.orcaPublicURL.JoinPath(proxyBasePath), httpClient.Transport, false, true)
		carePlanServiceProxy.ServeHTTP(writer, request)
	}))

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
	proxyBasePath := basePath + "/ehr/fhir"
	proxy := coolfhir.NewProxy("App->EHR FHIR proxy", clientFactory.BaseURL, proxyBasePath,
		s.orcaPublicURL.JoinPath(proxyBasePath), clientFactory.Client, false, false)

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

	if s.ehrFhirProxy == nil {
		return coolfhir.BadRequest("EHR API is not supported")
	}

	log.Ctx(request.Context()).Debug().Msg("Handling external FHIR API request")

	//TODO: Check if this is also related to not having the local cp
	_, err := s.authorizeScpMember(request)
	if err != nil {
		return err
	}
	s.ehrFhirProxy.ServeHTTP(writer, request)
	return nil
}

// proxyToAllCareTeamMembers is a convenience façade method that can be used proxy the request to all CPC nodes localized from the Shared CarePlan.participants.
func (s *Service) proxyToAllCareTeamMembers(writer http.ResponseWriter, request *http.Request) error {
	carePlanURLValue := request.Header[carePlanURLHeaderKey]
	if len(carePlanURLValue) != 1 {
		return coolfhir.BadRequest(fmt.Sprintf("%s header must only contain one value", carePlanURLHeaderKey))
	}
	log.Debug().Msg("Handling BgZ FHIR API request carePlanURL: " + carePlanURLValue[0])

	// Get the CPS base URL from the X-SCP-Context header (everything before /CarePlan/<id>).
	cpsBaseURL, carePlanRef, err := coolfhir.ParseExternalLiteralReference(carePlanURLValue[0], "CarePlan")
	if err != nil {
		return coolfhir.BadRequestError(fmt.Errorf("invalid %s header: %w", carePlanURLHeaderKey, err))
	}
	cpsFHIRClient, _, err := s.createFHIRClientForURL(request.Context(), cpsBaseURL)
	if err != nil {
		return err
	}

	var carePlan fhir.CarePlan

	err = cpsFHIRClient.ReadWithContext(request.Context(), carePlanRef, &carePlan)
	if err != nil {
		return fmt.Errorf("failed to get CarePlan %s for X-SCP-Context: %w", carePlanRef, err)
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	if err != nil {
		return fmt.Errorf("failed to resolve CareTeam (carePlan=%s): %w", carePlanRef, err)
	}

	// Collect participants. Use a map to ensure we don't send the same request to the same participant multiple times.
	participantIdentifiers := make(map[string]fhir.Identifier)
	for _, participant := range careTeam.Participant {
		if !coolfhir.IsLogicalReference(participant.Member) {
			continue
		}
		activeMember, err := coolfhir.ValidateCareTeamParticipantPeriod(participant, time.Now())
		if err != nil {
			log.Ctx(request.Context()).Warn().Err(err).Msg("Failed to validate CareTeam participant period")
		} else if activeMember {
			participantIdentifiers[coolfhir.ToString(participant.Member.Identifier)] = *participant.Member.Identifier
		}
	}
	if len(participantIdentifiers) == 0 {
		return coolfhir.NewErrorWithCode("no active participants found in CareTeam", http.StatusNotFound)
	}

	localIdentities, err := s.profile.Identities(request.Context())
	if err != nil {
		return err
	}

	type queryTarget struct {
		fhirBaseURL *url.URL
		httpClient  *http.Client
	}
	var queryTargets []queryTarget
	for _, identifier := range participantIdentifiers {
		// Don't fetch data from own endpoint, since we don't support querying from multiple endpoints yet
		if coolfhir.HasIdentifier(identifier, coolfhir.OrganizationIdentifiers(localIdentities)...) {
			continue
		}
		fhirEndpoints, err := s.profile.CsdDirectory().LookupEndpoint(request.Context(), &identifier, profile.FHIRBaseURLEndpointName)
		if err != nil {
			return fmt.Errorf("failed to lookup FHIR base URL for participant %s: %w", coolfhir.ToString(identifier), err)
		}
		for _, fhirEndpoint := range fhirEndpoints {
			parsedEndpointAddress, err := url.Parse(fhirEndpoint.Address)
			if err != nil {
				// TODO: When querying multiple participants, we should continue with the next participant instead of returning an error, and return an entry for the OperationOutcome
				return fmt.Errorf("failed to parse FHIR base URL for participant %s: %w", coolfhir.ToString(identifier), err)
			}
			httpClient, err := s.profile.HttpClient(request.Context(), identifier)
			if err != nil {
				return fmt.Errorf("failed to create HTTP client for participant %s: %w", coolfhir.ToString(identifier), err)
			}
			queryTargets = append(queryTargets, queryTarget{fhirBaseURL: parsedEndpointAddress, httpClient: httpClient})
		}
	}

	// Could add more information in the future with OperationOutcome messages
	if len(queryTargets) == 0 {
		return errors.New("didn't find any queryable FHIR endpoints for any active participant in related CareTeams")
	}
	if len(queryTargets) > 1 {
		// TODO: In this case, we need to aggregate the results from multiple endpoints
		return errors.New("found multiple queryable FHIR endpoints for active participants in related CareTeams, currently not supported")
	}
	const proxyBasePath = basePath + "/aggregate/fhir/"
	for _, target := range queryTargets {
		fhirProxy := coolfhir.NewProxy("EHR(local)->EHR(external) FHIR proxy", target.fhirBaseURL, proxyBasePath, s.orcaPublicURL.JoinPath(proxyBasePath), target.httpClient.Transport, true, true)
		fhirProxy.ServeHTTP(writer, request)
	}
	return nil
}

// TODO: Fix the logic in this method, it doesn't work as intended
func (s Service) authorizeScpMember(request *http.Request) (*ScpValidationResult, error) {
	// Authorize requester before proxying FHIR request
	// Data holder must verify that the requester is part of the CareTeam by checking the URA
	// Validate by retrieving the CarePlan from CPS, use URA in provided token to validate against CareTeam
	// CarePlan should be provided in X-Scp-Context header
	carePlanURLValue := request.Header[carePlanURLHeaderKey]
	if len(carePlanURLValue) == 0 {
		return nil, coolfhir.BadRequest(fmt.Sprintf("%s header must be set", carePlanURLHeaderKey))
	}
	if len(carePlanURLValue) > 1 {
		return nil, coolfhir.BadRequest(fmt.Sprintf("%s header can't contain multiple values", carePlanURLHeaderKey))
	}
	carePlanURL := carePlanURLValue[0]

	cpsBaseURL, carePlanRef, err := coolfhir.ParseExternalLiteralReference(carePlanURL, "CarePlan")
	if err != nil {
		return nil, coolfhir.BadRequest("specified SCP context header does not refer to a CarePlan")
	} else {
		_, _, err = coolfhir.ParseLocalReference(carePlanRef)
		if err != nil {
			return nil, coolfhir.BadRequest("specified SCP context header does not refer to a CarePlan")
		}
	}

	cpsFHIRClient, _, err := s.createFHIRClientForURL(request.Context(), cpsBaseURL)
	if err != nil {
		return nil, err
	}

	var carePlan fhir.CarePlan

	err = cpsFHIRClient.ReadWithContext(request.Context(), carePlanRef, &carePlan)
	if err != nil {
		var outcomeError fhirclient.OperationOutcomeError

		if errors.As(err, &outcomeError); err != nil {
			return nil, coolfhir.NewErrorWithCode(outcomeError.Error(), outcomeError.HttpStatusCode)
		}

		return nil, err
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	if err != nil {
		return nil, err
	}

	// Validate CareTeam participants against requester
	principal, err := auth.PrincipalFromContext(request.Context())
	if err != nil {
		return nil, err
	}

	// get the CareTeamParticipant, then check that it is active
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeam, principal.Organization.Identifier)
	if participant == nil {
		return nil, coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}

	isValid, err := coolfhir.ValidateCareTeamParticipantPeriod(*participant, time.Now())
	if err != nil {
		return nil, err
	}

	if !isValid {
		return nil, coolfhir.NewErrorWithCode("requester does not have access to resource", http.StatusForbidden)
	}
	return &ScpValidationResult{
		carePlan:  &carePlan,
		careTeams: &[]fhir.CareTeam{*careTeam},
	}, nil
}

func (s Service) handleGetContext(response http.ResponseWriter, _ *http.Request, session *user.SessionData) {
	contextData := struct {
		Patient          string `json:"patient"`
		ServiceRequest   string `json:"serviceRequest"`
		Practitioner     string `json:"practitioner"`
		PractitionerRole string `json:"practitionerRole"`
		Task             string `json:"task"`
		TaskIdentifier   string `json:"taskIdentifier"`
	}{
		Patient:          session.StringValues["patient"],
		ServiceRequest:   session.StringValues["serviceRequest"],
		Practitioner:     session.StringValues["practitioner"],
		PractitionerRole: session.StringValues["practitionerRole"],
		Task:             session.StringValues["task"],
		TaskIdentifier:   session.StringValues["taskIdentifier"],
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
	sender, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return err
	}

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

	log.Ctx(ctx).Info().Msgf("Received notification: Reference %s, Type: %s", *focusReference.Reference, *focusReference.Type)

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
	fhirBaseURL, _, err := coolfhir.ParseExternalLiteralReference(resourceUrl, *focusReference.Type)
	if err != nil {
		return err
	}

	fhirClient, _, err := s.createFHIRClientForIdentifier(ctx, fhirBaseURL, sender.Organization.Identifier[0])
	if err != nil {
		return err
	}
	var focusResource any
	switch *focusReference.Type {
	case "Task":
		var task fhir.Task
		err = fhirClient.Read(*focusReference.Reference, &task)
		if err != nil {
			return err
		}
		focusResource = task
		// TODO: How to differentiate between create and update? (Currently we only use Create in CPS. There is code for Update but nothing calls it)
		// TODO: Move this to a event.Handler implementation
		err = s.handleTaskNotification(ctx, fhirClient, &task)
		rejection := new(TaskRejection)
		if errors.As(err, &rejection) || errors.As(err, rejection) {
			if err := s.rejectTask(ctx, fhirClient, task, *rejection); err != nil {
				// TODO: what to do here?
				log.Ctx(ctx).Err(err).Msgf("Failed to reject task (id=%s, reason=%s)", *task.Id, rejection.FormatReason())
			}
		} else if err != nil {
			return err
		}
	case "CarePlan":
		var carePlan fhir.CarePlan
		err = fhirClient.Read(*focusReference.Reference, &carePlan)
		if err != nil {
			return err
		}
		focusResource = carePlan
	default:
		log.Ctx(ctx).Debug().Msgf("No handler for notification of type %s, ignoring", *focusReference.Type)
	}
	// TODO: Not sure if we should return an error here
	return s.eventManager.Notify(ctx, *focusReference.Type, events.Instance{
		FHIRResource:       focusResource,
		FHIRResourceSource: *focusReference.Reference,
	})
}

func (s Service) rejectTask(ctx context.Context, client fhirclient.Client, task fhir.Task, rejection TaskRejection) error {
	log.Ctx(ctx).Info().Msgf("Rejecting task (id=%s, reason=%s)", *task.Id, rejection.FormatReason())
	task.Status = fhir.TaskStatusRejected
	task.StatusReason = &fhir.CodeableConcept{
		Text: to.Ptr(rejection.FormatReason()),
	}
	return client.UpdateWithContext(ctx, "Task/"+*task.Id, task, &task)
}

func (s Service) createFHIRClientForURL(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error) {
	// We only have the FHIR base URL, we need to read the CapabilityStatement to find out the Authorization Server URL
	identifier := fhir.Identifier{
		System: to.Ptr("https://build.fhir.org/http.html#root"),
		Value:  to.Ptr(fhirBaseURL.String()),
	}
	return s.createFHIRClientForIdentifier(ctx, fhirBaseURL, identifier)
}

func (s Service) createFHIRClientForIdentifier(ctx context.Context, fhirBaseURL *url.URL, identifier fhir.Identifier) (fhirclient.Client, *http.Client, error) {
	httpClient, err := s.profile.HttpClient(ctx, identifier)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create HTTP client (identifier=%s): %w", coolfhir.ToString(identifier), err)
	}
	return fhirClientFactory(fhirBaseURL, httpClient), httpClient, nil
}

func createFHIRClient(fhirBaseURL *url.URL, httpClient *http.Client) fhirclient.Client {
	return fhirclient.New(fhirBaseURL, httpClient, coolfhir.Config())
}
