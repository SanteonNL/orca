//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/messaging"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/ehr"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/sse"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/pubsub"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const basePath = "/cpc"

// scpIdentifierHeaderKey specifies the HTTP request header used to specify the identifier of the external entity (care organization),
// whose FHIR API should be queried. The identifier is in the form of <system>|<value>.
const scpEntityIdentifierHeaderKey = "X-Scp-Entity-Identifier"

// scpFHIRBaseURL specifies the HTTP request header used to specify the FHIR base URL, secured through SCP,
// which should be queried.
const scpFHIRBaseURL = "X-Scp-Fhir-Url"

// carePlanURLHeaderKey specifies the HTTP request header used to specify the SCP context, which is a reference to a FHIR CarePlan. Authorization is evaluated according to this CarePlan.
// The header may also be provided as X-SCP-Context, which will be canonicalized to X-Scp-Context by the Golang HTTP client.
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
	sessionManager *user.SessionManager[session.Data],
	messageBroker messaging.Broker,
	eventManager events.Manager,
	ehrFhirProxy coolfhir.HttpProxy,
	localCarePlanServiceURL *url.URL,
	httpHandler http.Handler) (*Service, error) {

	fhirURL, _ := url.Parse(config.FHIR.BaseURL)

	localFhirStoreTransport, _, err := coolfhir.NewAuthRoundTripper(config.FHIR, coolfhir.Config())
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	if config.HealthDataViewEndpointEnabled {
		if ehrFhirProxy == nil {
			ehrFhirProxy = coolfhir.NewProxy("App->EHR (DataView)", fhirURL, basePath+"/fhir", orcaPublicURL.JoinPath(basePath, "fhir"), localFhirStoreTransport, false, false)
		}
	}

	// Initialize workflow provider, which is used to select FHIR Questionnaires by the Task Filler engine
	var workflowProvider taskengine.WorkflowProvider
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
		sseService:                    sse.New(),
		httpHandler:                   httpHandler,
	}
	if config.OIDCProvider.Enabled {
		result.oidcProvider, err = oidc.New(globals.StrictMode, orcaPublicURL.JoinPath(basePath), config.OIDCProvider)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}
	}

	result.createFHIRClientForURL = result.defaultCreateFHIRClientForURL
	if config.TaskFiller.TaskAcceptedBundleTopic != "" {
		result.notifier, err = ehr.NewNotifier(eventManager, messageBroker, messaging.Entity{Name: config.TaskFiller.TaskAcceptedBundleTopic}, result.createFHIRClientForURL)
		if err != nil {
			return nil, fmt.Errorf("TaskEngine: failed to create EHR notifier: %w", err)
		}
		log.Ctx(ctx).Info().Msgf("TaskEngine: created EHR notifier for topic %s", config.TaskFiller.TaskAcceptedBundleTopic)
	}
	pubsub.DefaultSubscribers.FhirSubscriptionNotify = result.handleNotification
	return result, nil
}

type Service struct {
	config         Config
	profile        profile.Provider
	orcaPublicURL  *url.URL
	SessionManager *user.SessionManager[session.Data]
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
	sseService                    *sse.Service
	createFHIRClientForURL        func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
	oidcProvider                  *oidc.Service
	httpHandler                   http.Handler
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	if s.oidcProvider != nil {
		mux.HandleFunc(basePath+"/login", s.withSession(s.oidcProvider.HandleLogin))
		mux.Handle(basePath+"/", http.StripPrefix(basePath, s.oidcProvider))
	}

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
			return coolfhir.BadRequest("bundle type not supported: %s", notification.Type.String())
		}
		if err := s.handleNotification(httpRequest.Context(), (*coolfhir.SubscriptionNotification)(&notification)); err != nil {
			return err
		}
		return nil
	}
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

	//
	// This is a special endpoint, used by other SCP-nodes to discovery applications.
	//
	mux.HandleFunc("GET "+basePath+"/fhir/Endpoint", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		if len(request.URL.Query()) > 0 {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), coolfhir.BadRequest("search parameters are not supported on this endpoint"), fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		}
		bundle := coolfhir.BundleBuilder{}
		bundle.Type = fhir.BundleTypeSearchset
		endpoints := make(map[string]fhir.Endpoint)
		endpointNames := make([]string, 0)
		for _, appConfig := range s.config.AppLaunch.External {
			endpoint := fhir.Endpoint{
				Status: fhir.EndpointStatusActive,
				ConnectionType: fhir.Coding{
					System: to.Ptr("http://santeonnl.github.io/shared-care-planning/endpoint-connection-type"),
					Code:   to.Ptr("web-oauth2"),
				},
				PayloadType: []fhir.CodeableConcept{
					{
						Coding: []fhir.Coding{
							{
								System: to.Ptr("http://santeonnl.github.io/shared-care-planning/endpoint-payload-type"),
								Code:   to.Ptr("web-application"),
							},
						},
					},
				},
				Name:    to.Ptr(appConfig.Name),
				Address: appConfig.URL,
			}
			endpoints[appConfig.Name] = endpoint
			endpointNames = append(endpointNames, appConfig.Name)
		}
		// Stable order for sanity and easier testing
		slices.Sort(endpointNames)
		for _, name := range endpointNames {
			bundle.Append(endpoints[name], nil, nil)
		}
		coolfhir.SendResponse(writer, http.StatusOK, bundle, nil)
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
	// This endpoint is used by the EHR and ORCA Frontend to query the FHIR API of a remote SCP-node.
	// The remote SCP-node to query can be specified using the following HTTP headers:
	// - X-Scp-Entity-Identifier: Uses the identifier of the SCP-node to query (in the form of <system>|<value>), to resolve the registered FHIR base URL
	mux.HandleFunc(basePath+"/external/fhir/{rest...}", s.withSessionOrBearerToken(func(writer http.ResponseWriter, request *http.Request) {
		fhirBaseURL, httpClient, err := s.createFHIRClientForExternalRequest(request.Context(), request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
		const proxyBasePath = basePath + "/external/fhir/"
		fhirProxy := coolfhir.NewProxy("EHR(local)->EHR(external) FHIR proxy", fhirBaseURL, proxyBasePath, s.orcaPublicURL.JoinPath(proxyBasePath), httpClient.Transport, true, true)
		fhirProxy.ServeHTTP(writer, request)
	}))
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
	// Allow the front-end to subscribe to specific Task updates via Server-Sent Events (SSE)
	mux.HandleFunc("GET "+basePath+"/subscribe/fhir/Task/{id}", s.withSession(s.handleSubscribeToTask))

	// Logout endpoint
	mux.HandleFunc("/logout", s.withSession(func(writer http.ResponseWriter, request *http.Request, _ *session.Data) {
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
func (s Service) withSession(next func(response http.ResponseWriter, request *http.Request, session *session.Data)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		sessionData := s.SessionManager.Get(request)
		if sessionData == nil {
			http.Error(response, "no session found", http.StatusUnauthorized)
			return
		}
		next(response, request, sessionData)
	}
}

// handleProxyAppRequestToEHR handles a request from the CPC application (e.g. Frontend), forwarding it to the local EHR's FHIR API.
func (s Service) handleProxyAppRequestToEHR(writer http.ResponseWriter, request *http.Request, sessionData *session.Data) {
	clientFactory := clients.Factories[sessionData.FHIRLauncher](sessionData.LauncherProperties)
	proxyBasePath := basePath + "/ehr/fhir"
	proxy := coolfhir.NewProxy("App->EHR FHIR proxy", clientFactory.BaseURL, proxyBasePath,
		s.orcaPublicURL.JoinPath(proxyBasePath), clientFactory.Client, false, false)

	resourcePath := request.PathValue("rest")
	// If the requested resource is cached in the session, directly return it. This is used to support resources that are required (e.g. by Frontend), but not provided by the EHR.
	// E.g., ChipSoft HiX doesn't provide ServiceRequest and Practitioner as FHIR resources, so whatever there is, is converted to FHIR and cached in the session.
	if resource := sessionData.GetByPath(resourcePath); resource != nil && resource.Resource != nil {
		coolfhir.SendResponse(writer, http.StatusOK, *resource.Resource)
	} else {
		proxy.ServeHTTP(writer, request)
	}
}

func (s Service) handleSubscribeToTask(writer http.ResponseWriter, request *http.Request, sessionData *session.Data) {
	if sessionData.TaskIdentifier == nil {
		coolfhir.WriteOperationOutcomeFromError(request.Context(), coolfhir.BadRequest("No taskIdentifier found in session"), fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}

	sessionTaskIdentifier, err := coolfhir.TokenToIdentifier(*sessionData.TaskIdentifier)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(request.Context(), coolfhir.BadRequest("Invalid taskIdentifier in session: %v", err), fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}

	if s.localCarePlanServiceUrl == nil {
		coolfhir.WriteOperationOutcomeFromError(request.Context(), coolfhir.BadRequest("No local CarePlanService configured - cannot verify Task identifiers"), fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}

	//Ensure the sessions taskIdentifier matches the requested task
	cpsClient, _, err := s.createFHIRClientForURL(request.Context(), s.localCarePlanServiceUrl)
	if err != nil {
		log.Ctx(request.Context()).Err(err).Msgf("Failed to create local CarePlanService FHIR client: %v", err)
		coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "Failed to create local SCP client", writer)
		return
	}

	id := request.PathValue("id")
	var task fhir.Task
	err = cpsClient.ReadWithContext(request.Context(), fmt.Sprintf("Task/%s", id), &task)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}

	//CPS Task found, make sure the identifier from the session exists on the Task
	found := false
	if task.Identifier != nil {
		for _, identifier := range task.Identifier {
			if identifier.Value != nil && *identifier.Value == *sessionTaskIdentifier.Value && *identifier.System == *sessionTaskIdentifier.System {
				found = true
				break
			}
		}
	}

	if !found {
		coolfhir.WriteOperationOutcomeFromError(
			request.Context(),
			coolfhir.BadRequest("Task identifier does not match the taskIdentifier in the session"),
			fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path),
			writer,
		)
		return
	}

	// Subscribed task contains the taskIdentifier from the session, so we can subscribe to the task
	s.sseService.ServeHTTP(fmt.Sprintf("Task/%s", id), writer, request)
}

// handleProxyExternalRequestToEHR handles a request from an external SCP-node (e.g. CarePlanContributor), forwarding it to the local EHR's FHIR API.
// This is typically used by remote parties to retrieve patient data from the local EHR.
func (s Service) handleProxyExternalRequestToEHR(writer http.ResponseWriter, request *http.Request) error {

	if s.ehrFhirProxy == nil {
		return coolfhir.BadRequest("EHR API is not supported")
	}

	log.Ctx(request.Context()).Debug().Msg("Handling external FHIR API request")

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
		return coolfhir.BadRequest("%s header must only contain one value", carePlanURLHeaderKey)
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
		return nil, coolfhir.BadRequest("%s header must be set", carePlanURLHeaderKey)
	}
	if len(carePlanURLValue) > 1 {
		return nil, coolfhir.BadRequest("%s header can't contain multiple values", carePlanURLHeaderKey)
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

func (s Service) handleGetContext(response http.ResponseWriter, _ *http.Request, sessionData *session.Data) {
	contextData := struct {
		Patient          string  `json:"patient"`
		ServiceRequest   string  `json:"serviceRequest"`
		Practitioner     string  `json:"practitioner"`
		PractitionerRole string  `json:"practitionerRole"`
		Task             string  `json:"task"`
		TaskIdentifier   *string `json:"taskIdentifier"`
	}{
		Patient:          to.Empty(sessionData.GetByType("Patient")).Path,
		ServiceRequest:   to.Empty(sessionData.GetByType("ServiceRequest")).Path,
		Practitioner:     to.Empty(sessionData.GetByType("Practitioner")).Path,
		PractitionerRole: to.Empty(sessionData.GetByType("PractitionerRole")).Path,
		Task:             to.Empty(sessionData.GetByType("Task")).Path,
		TaskIdentifier:   sessionData.TaskIdentifier,
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
	switch *focusReference.Type {
	case "Task":
		var task fhir.Task
		err = fhirClient.Read(*focusReference.Reference, &task)
		if err != nil {
			return err
		}
		//insert the meta.source - can be used to determine the X-Scp-Context
		if task.Meta == nil {
			task.Meta = &fhir.Meta{}
		}

		if task.Meta.Source != nil && *task.Meta.Source != resourceUrl {
			log.Ctx(ctx).Warn().Msgf("Task (id=%s) already has a source (%s), overwriting it to (%s)", *task.Id, *task.Meta.Source, resourceUrl)
		}

		task.Meta.Source = &resourceUrl

		// TODO: How to differentiate between create and update? (Currently we only use Create in CPS. There is code for Update but nothing calls it)
		// TODO: Move this to a event.Handler implementation
		err = s.publishTaskToSse(ctx, &task)
		if err != nil {
			//gracefully log the error, but continue processing the notification
			log.Ctx(ctx).Err(err).Msgf("Failed to publish task (id=%s) to SSE", *task.Id)
		}

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
	default:
		log.Ctx(ctx).Debug().Msgf("No handler for notification of type %s, ignoring", *focusReference.Type)
	}
	return nil
}

func (s Service) publishTaskToSse(ctx context.Context, task *fhir.Task) error {
	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Check if the Task is a subTask
	var parentTaskReference string
	if len(task.PartOf) > 0 {
		for _, reference := range task.PartOf {
			if reference.Reference != nil && strings.HasPrefix(*reference.Reference, "Task/") {
				parentTaskReference = *reference.Reference
				break
			}
		}
	}

	if parentTaskReference != "" {
		s.sseService.Publish(ctx, parentTaskReference, string(data))
	} else {
		s.sseService.Publish(ctx, fmt.Sprintf("Task/%s", *task.Id), string(data))
	}

	return nil
}

func (s Service) rejectTask(ctx context.Context, client fhirclient.Client, task fhir.Task, rejection TaskRejection) error {
	log.Ctx(ctx).Info().Msgf("Rejecting task (id=%s, reason=%s)", *task.Id, rejection.FormatReason())
	task.Status = fhir.TaskStatusRejected
	task.StatusReason = &fhir.CodeableConcept{
		Text: to.Ptr(rejection.FormatReason()),
	}
	return client.UpdateWithContext(ctx, "Task/"+*task.Id, task, &task)
}

func (s Service) defaultCreateFHIRClientForURL(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error) {
	// We only have the FHIR base URL, we need to read the CapabilityStatement to find out the Authorization Server URL
	identifier := fhir.Identifier{
		System: to.Ptr("https://build.fhir.org/http.html#root"),
		Value:  to.Ptr(fhirBaseURL.String()),
	}
	return s.createFHIRClientForIdentifier(ctx, fhirBaseURL, identifier)
}

// createFHIRClientForExternalRequest creates a FHIR client for a request that should be proxied to an external SCP-node's FHIR API.
// It derives the remote SCP-node from the HTTP request headers:
// - X-Scp-Entity-Identifier: Uses the identifier of the SCP-node to query (in the form of <system>|<value>), to resolve the registered FHIR base URL.
// - X-Scp-Fhir-Url: Uses the FHIR base URL directly.
func (s Service) createFHIRClientForExternalRequest(ctx context.Context, request *http.Request) (*url.URL, *http.Client, error) {
	var httpClient *http.Client
	var fhirBaseURL *url.URL
	for _, header := range []string{scpEntityIdentifierHeaderKey, scpFHIRBaseURL} {
		headerValue := request.Header.Get(header)
		if headerValue == "" {
			continue
		}
		switch header {
		case scpFHIRBaseURL:
			var err error
			if headerValue == "local-cps" ||
				(s.localCarePlanServiceUrl != nil && headerValue == s.localCarePlanServiceUrl.String()) {
				// Targeted FHIR API is local CPS, either through 'local-cps' or because the target URL matches the local CPS URL
				fhirBaseURL = s.localCarePlanServiceUrl
				if fhirBaseURL == nil {
					return nil, nil, fmt.Errorf("%s: no local CarePlanService", header)
				}
				httpClient = s.httpClientForLocalCPS(httpClient)
			} else {
				fhirBaseURL, err = s.parseFHIRBaseURL(headerValue)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: %w", header, err)
				}
				_, httpClient, err = s.createFHIRClientForURL(ctx, fhirBaseURL)
				if err != nil {
					return nil, nil, fmt.Errorf("%s: failed to create HTTP client: %w", header, err)
				}
			}
			break
		case scpEntityIdentifierHeaderKey:
			// The header value is in the form of <system>|<value>
			identifier, err := coolfhir.TokenToIdentifier(headerValue)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: invalid identifier (value=%s): %w", header, headerValue, err)
			}
			endpoints, err := s.profile.CsdDirectory().LookupEndpoint(ctx, identifier, profile.FHIRBaseURLEndpointName)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: failed to lookup FHIR base URL (identifier=%s): %w", header, headerValue, err)
			}
			if len(endpoints) != 1 {
				return nil, nil, fmt.Errorf("%s: expected one FHIR base URL, got %d (identifier=%s)", header, len(endpoints), headerValue)
			}
			fhirBaseURL, err = s.parseFHIRBaseURL(endpoints[0].Address)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: registered FHIR base URL is invalid (identifier=%s): %w", header, headerValue, err)
			}
			_, httpClient, err = s.createFHIRClientForIdentifier(ctx, fhirBaseURL, *identifier)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: failed to create HTTP client (identifier=%s): %w", header, headerValue, err)
			}
			break
		}
	}
	if httpClient == nil || fhirBaseURL == nil {
		return nil, nil, coolfhir.BadRequest("can't determine the external SCP-node to query from the HTTP request headers")
	}
	return fhirBaseURL, httpClient, nil
}

func (s Service) httpClientForLocalCPS(httpClient *http.Client) *http.Client {
	httpClient = &http.Client{Transport: internalDispatchHTTPRoundTripper{
		profile: s.profile,
		handler: s.httpHandler,
		matcher: func(request *http.Request) bool {
			return true
		},
	}}
	return httpClient
}

func (s Service) parseFHIRBaseURL(fhirBaseURL string) (*url.URL, error) {
	parsedURL, err := url.Parse(fhirBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid FHIR base URL: %s", fhirBaseURL)
	}
	if err := s.validateFHIRBaseURL(parsedURL); err != nil {
		return nil, err
	}
	return parsedURL, nil
}

func (s Service) validateFHIRBaseURL(fhirBaseURL *url.URL) error {
	if !fhirBaseURL.IsAbs() || (fhirBaseURL.Scheme != "http" && fhirBaseURL.Scheme != "https") {
		return fmt.Errorf("invalid FHIR base URL: %s", fhirBaseURL)
	}
	if globals.StrictMode && fhirBaseURL.Scheme != "https" {
		return fmt.Errorf("invalid FHIR base URL: %s (only HTTPS is allowed in strict mode)", fhirBaseURL)
	}
	return nil
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
