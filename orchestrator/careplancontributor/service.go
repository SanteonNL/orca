//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/op"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

const basePathWithTenant = basePath + "/{tenant}"
const basePath = "/cpc"
const tracerName = "careplancontributor"

func FHIRBaseURL(tenantID string, orcaBaseURL *url.URL) *url.URL {
	return orcaBaseURL.JoinPath(basePath, tenantID, "fhir")
}

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
	tenants tenants.Config,
	profile profile.Provider,
	orcaPublicURL *url.URL,
	sessionManager *user.SessionManager[session.Data],
	messageBroker messaging.Broker,
	eventManager events.Manager,
	cpsEnabled bool,
	httpHandler http.Handler) (*Service, error) {
	ctx := context.Background()
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
		tenants:                       tenants,
		orcaPublicURL:                 orcaPublicURL,
		cpsEnabled:                    cpsEnabled,
		SessionManager:                sessionManager,
		profile:                       profile,
		ehrFHIRProxyByTenant:          make(map[string]coolfhir.HttpProxy),
		ehrFHIRClientByTenant:         make(map[string]fhirclient.Client),
		workflows:                     workflowProvider,
		healthdataviewEndpointEnabled: config.HealthDataViewEndpointEnabled,
		eventManager:                  eventManager,
		httpHandler:                   httpHandler,
	}
	var err error
	if config.OIDC.Provider.Enabled {
		result.oidcProvider, err = op.New(globals.StrictMode, orcaPublicURL.JoinPath(basePath), config.OIDC.Provider)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}
	}
	if config.OIDC.RelyingParty.Enabled {
		result.tokenClient, err = rp.NewClient(ctx, &config.OIDC.RelyingParty)
		if err != nil {
			return nil, fmt.Errorf("failed to create ADB2C client: %w", err)
		}
	}

	result.createFHIRClientForURL = result.defaultCreateFHIRClientForURL
	if config.TaskFiller.TaskAcceptedBundleTopic != "" {
		result.notifier, err = ehr.NewNotifier(eventManager, messageBroker, tenants, messaging.Entity{Name: config.TaskFiller.TaskAcceptedBundleTopic}, config.TaskFiller.TaskAcceptedBundleEndpoint, result.createFHIRClientForURL)
		if err != nil {
			return nil, fmt.Errorf("TaskEngine: failed to create EHR notifier: %w", err)
		}
		log.Ctx(ctx).Info().Msgf("TaskEngine: created EHR notifier for topic %s", config.TaskFiller.TaskAcceptedBundleTopic)
	}
	pubsub.DefaultSubscribers.FhirSubscriptionNotify = result.handleNotification

	if err = result.initializeAppLaunches(sessionManager, globals.StrictMode); err != nil {
		return nil, fmt.Errorf("failed to initialize AppLaunch services: %w", err)
	}
	return result, nil
}

type Service struct {
	config                        Config
	tenants                       tenants.Config
	profile                       profile.Provider
	orcaPublicURL                 *url.URL
	SessionManager                *user.SessionManager[session.Data]
	ehrFHIRProxyByTenant          map[string]coolfhir.HttpProxy
	ehrFHIRClientByTenant         map[string]fhirclient.Client
	workflows                     taskengine.WorkflowProvider
	healthdataviewEndpointEnabled bool
	notifier                      ehr.Notifier
	eventManager                  events.Manager
	createFHIRClientForURL        func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
	oidcProvider                  *op.Service
	httpHandler                   http.Handler
	tokenClient                   *rp.Client
	appLaunches                   []applaunch.Service
	cpsEnabled                    bool
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	if s.oidcProvider != nil {
		mux.HandleFunc(basePath+"/login", s.withSession(s.oidcProvider.HandleLogin))
		mux.Handle(basePath+"/", http.StripPrefix(basePath, s.oidcProvider))
	}

	//
	// The section below defines endpoints specified by Shared Care Planning.
	// These are secured through the profile (e.g. Nuts access tokens)
	//
	handleBundle := func(httpRequest *http.Request) (*fhir.Bundle, error) {
		var bundle fhir.Bundle
		if err := json.NewDecoder(httpRequest.Body).Decode(&bundle); err != nil {
			return nil, coolfhir.BadRequest("failed to decode bundle: %w", err)
		}
		if coolfhir.IsSubscriptionNotification(&bundle) {
			if err := s.handleNotification(httpRequest.Context(), (*coolfhir.SubscriptionNotification)(&bundle)); err != nil {
				return nil, err
			}
			return &fhir.Bundle{Type: fhir.BundleTypeHistory}, nil
		} else if bundle.Type == fhir.BundleTypeBatch {
			return s.handleBatch(httpRequest, bundle)
		}
		return nil, coolfhir.BadRequest("bundle type not supported: %s", bundle.Type.String())
	}
	mux.HandleFunc("POST "+basePathWithTenant+"/fhir", s.tenants.HttpHandler(s.profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
		if bundle, err := handleBundle(request); err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/CreateBundle", writer)
		} else {
			coolfhir.SendResponse(writer, http.StatusOK, bundle)
		}
	})))
	mux.HandleFunc("POST "+basePathWithTenant+"/fhir/", s.tenants.HttpHandler(s.profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
		if bundle, err := handleBundle(request); err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/CreateBundle", writer)
		} else {
			coolfhir.SendResponse(writer, http.StatusOK, bundle)
		}
	})))

	//
	// This is a special endpoint, used by other SCP-nodes to discovery applications.
	//
	mux.HandleFunc("GET "+basePathWithTenant+"/fhir/Endpoint", s.tenants.HttpHandler(s.profile.Authenticator(func(writer http.ResponseWriter, request *http.Request) {
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
	})))
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
	mux.HandleFunc("GET "+basePathWithTenant+"/fhir/{resourceType}/{id}", s.tenants.HttpHandler(s.profile.Authenticator(proxyGetOrSearchHandler)))
	mux.HandleFunc("POST "+basePathWithTenant+"/fhir/{resourceType}/_search", s.tenants.HttpHandler(s.profile.Authenticator(proxyGetOrSearchHandler)))
	mux.HandleFunc("GET "+basePathWithTenant+"/fhir/{resourceType}", s.tenants.HttpHandler(s.profile.Authenticator(proxyGetOrSearchHandler)))
	//
	// The section below defines endpoints used for integrating the local EHR with ORCA.
	// They are NOT specified by SCP. Authorization is specific to the local EHR.
	//
	// This endpoint is used by the EHR and ORCA Frontend to query the FHIR API of a remote SCP-node.
	// The remote SCP-node to query can be specified using the following HTTP headers:
	// - X-Scp-Entity-Identifier: Uses the identifier of the SCP-node to query (in the form of <system>|<value>), to resolve the registered FHIR base URL
	mux.HandleFunc(basePathWithTenant+"/external/fhir/{rest...}", s.tenants.HttpHandler(s.withUserAuth(func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Extract relevant data from the bearer JWT
		fhirBaseURL, httpClient, err := s.createFHIRClientForExternalRequest(request.Context(), request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
		proxyBasePath, err := s.tenantBasePath(request.Context())
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
			return
		}
		proxyBasePath += "/external/fhir/"
		fhirProxy := coolfhir.NewProxy("EHR(local)->EHR(external) FHIR proxy", fhirBaseURL, proxyBasePath, s.orcaPublicURL.JoinPath(proxyBasePath), httpClient.Transport, true, true)
		fhirProxy.ServeHTTP(writer, request)
	})))
	mux.HandleFunc("GET "+basePath+"/context", s.withSession(s.handleGetContext))
	mux.HandleFunc(basePathWithTenant+"/ehr/fhir/{rest...}", s.tenants.HttpHandler(s.withSession(s.handleProxyAppRequestToEHR)))

	// Logout endpoint
	mux.HandleFunc("/logout", s.withSession(func(writer http.ResponseWriter, request *http.Request, _ *session.Data) {
		s.SessionManager.Destroy(writer, request)
		// If there is a 'Referer' value in the header, redirect to that URL
		if referer := request.Header.Get("Referer"); referer != "" {
			http.Redirect(writer, request, referer, http.StatusFound)
		} else {
			// This redirection will be handled by middleware in the frontend
			http.Redirect(writer, request, s.config.FrontendConfig.URL, http.StatusOK)
		}
	}))

	// App launch endpoints
	for _, appLaunch := range s.appLaunches {
		appLaunch.RegisterHandlers(mux)
	}
}

func (s *Service) initializeAppLaunches(sessionManager *user.SessionManager[session.Data], strictMode bool) error {
	frontendUrl, _ := url.Parse(s.config.FrontendConfig.URL)

	if s.config.AppLaunch.SmartOnFhir.Enabled {
		service, err := smartonfhir.New(s.config.AppLaunch.SmartOnFhir, sessionManager, s.orcaPublicURL, frontendUrl, strictMode)
		if err != nil {
			return fmt.Errorf("failed to create SMART on FHIR AppLaunch service: %w", err)
		}
		s.appLaunches = append(s.appLaunches, service)
	}
	if s.config.AppLaunch.Demo.Enabled {
		service := demo.New(sessionManager, s.config.AppLaunch.Demo, s.tenants, s.orcaPublicURL, frontendUrl, s.profile)
		s.appLaunches = append(s.appLaunches, service)
	}
	if s.config.AppLaunch.ZorgPlatform.Enabled {
		service, err := zorgplatform.New(sessionManager, s.config.AppLaunch.ZorgPlatform, s.tenants, s.orcaPublicURL.String(), frontendUrl, s.profile)
		if err != nil {
			return fmt.Errorf("failed to create Zorgplatform AppLaunch service: %w", err)
		}
		s.appLaunches = append(s.appLaunches, service)
	}
	for _, appLaunch := range s.appLaunches {
		proxies, fhirClients := appLaunch.CreateEHRProxies()
		for tenantID, proxy := range proxies {
			if _, exists := s.ehrFHIRProxyByTenant[tenantID]; exists {
				return fmt.Errorf("EHR FHIR proxy for tenant %s already exists", tenantID)
			}
			s.ehrFHIRProxyByTenant[tenantID] = proxy
		}
		for tenantID, fhirClient := range fhirClients {
			if _, exists := s.ehrFHIRClientByTenant[tenantID]; exists {
				return fmt.Errorf("EHR FHIR client for tenant %s already exists", tenantID)
			}
			s.ehrFHIRClientByTenant[tenantID] = fhirClient
		}
	}
	return nil
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
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		request.Context(),
		"handleProxyAppRequestToEHR",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", request.Method),
			attribute.String("http.url", request.URL.String()),
			attribute.String("operation.name", "CarePlanContributor/ProxyAppRequestToEHR"),
		),
	)
	defer span.End()

	start := time.Now()

	clientFactory := clients.Factories[sessionData.FHIRLauncher](sessionData.LauncherProperties)
	proxyBasePath, err := s.tenantBasePath(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}
	proxyBasePath += "/ehr/fhir"
	proxy := coolfhir.NewProxy("App->EHR FHIR proxy", clientFactory.BaseURL, proxyBasePath,
		s.orcaPublicURL.JoinPath(proxyBasePath), clientFactory.Client, false, false)

	resourcePath := request.PathValue("rest")
	// If the requested resource is cached in the session, directly return it. This is used to support resources that are required (e.g. by Frontend), but not provided by the EHR.
	// E.g., ChipSoft HiX doesn't provide ServiceRequest and Practitioner as FHIR resources, so whatever there is, is converted to FHIR and cached in the session.
	if resource := sessionData.GetByPath(resourcePath); resource != nil && resource.Resource != nil {
		coolfhir.SendResponse(writer, http.StatusOK, *resource.Resource)
	} else {
		proxy.ServeHTTP(writer, request.WithContext(ctx))
	}

	span.SetAttributes(
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
		attribute.String("fhir.resource_path", resourcePath),
	)
	span.SetStatus(codes.Ok, "")
}

// handleProxyExternalRequestToEHR handles a request from an external SCP-node (e.g. CarePlanContributor), forwarding it to the local EHR's FHIR API.
// This is typically used by remote parties to retrieve patient data from the local EHR.
func (s Service) handleProxyExternalRequestToEHR(writer http.ResponseWriter, request *http.Request) error {
	tenant, err := tenants.FromContext(request.Context())
	if err != nil {
		return err
	}

	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		request.Context(),
		"handleProxyExternalRequestToEHR",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", request.Method),
			attribute.String("http.url", request.URL.String()),
			attribute.String("operation.name", "CarePlanContributor/ProxyExternalRequestToEHR"),
		),
	)
	defer span.End()

	ehrProxy := s.ehrFHIRProxyByTenant[tenant.ID]
	if ehrProxy == nil {
		err := coolfhir.BadRequest("EHR API is not supported")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	start := time.Now()
	log.Ctx(ctx).Debug().Msg("Handling external FHIR API request")
	_, err = s.authorizeScpMember(request.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	ehrProxy.ServeHTTP(writer, request.WithContext(ctx))

	span.SetAttributes(
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")
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
	// Validate that the header value is a properly formatted SCP context URL
	_, err := s.parseFHIRBaseURL(carePlanURL)
	if err != nil {
		return nil, coolfhir.BadRequest("specified SCP context header is not a valid URL")
	}

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
		TenantID         string  `json:"tenantId"`
	}{
		Patient:          to.Empty(sessionData.GetByType("Patient")).Path,
		ServiceRequest:   to.Empty(sessionData.GetByType("ServiceRequest")).Path,
		Practitioner:     to.Empty(sessionData.GetByType("Practitioner")).Path,
		PractitionerRole: to.Empty(sessionData.GetByType("PractitionerRole")).Path,
		Task:             to.Empty(sessionData.GetByType("Task")).Path,
		TaskIdentifier:   sessionData.TaskIdentifier,
		TenantID:         sessionData.TenantID,
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(contextData)
}

func (s Service) withUserAuth(next func(response http.ResponseWriter, request *http.Request)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		// Session will be present for FE requests
		if s.SessionManager.Get(request) != nil {
			next(response, request)
			return
		}

		bearer := request.Header.Get("Authorization")
		// Static bearer token, not valid in prod
		if bearer == "Bearer "+s.config.StaticBearerToken {
			next(response, request)
			return
		}

		// Try to validate bearer token as adb2c jwt
		if s.tokenClient != nil {
			// Validate the token
			bearerToken := strings.TrimPrefix(bearer, "Bearer ")

			if bearerToken != "" {
				if _, err := s.tokenClient.ValidateToken(request.Context(), bearerToken); err != nil {
					log.Ctx(request.Context()).Err(err).Msg("Failed to validate ADB2C token")
					http.Error(response, "invalid bearer token", http.StatusUnauthorized)
					return
				}

				// TODO: additional validation: from the claims we need to at least extract the user ID so we can use that for BGZ data request

				next(response, request)
				return
			}
		}

		http.Error(response, "no user authentication found", http.StatusUnauthorized)
	}
}

func (s Service) handleNotification(ctx context.Context, resource any) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"handleNotification",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("operation.name", "CarePlanContributor/HandleNotification"),
		),
	)
	defer span.End()

	start := time.Now()

	sender, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	notification, ok := resource.(*coolfhir.SubscriptionNotification)
	if !ok {
		err := &coolfhir.ErrorWithCode{
			Message:    "failed to cast resource to notification",
			StatusCode: http.StatusBadRequest,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if notification == nil {
		err := &coolfhir.ErrorWithCode{
			Message:    "notification is nil",
			StatusCode: http.StatusInternalServerError,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	focusReference, err := notification.GetFocus()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	if focusReference == nil {
		err := &coolfhir.ErrorWithCode{
			Message:    "Notification focus not found",
			StatusCode: http.StatusUnprocessableEntity,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	if focusReference.Type == nil {
		err := &coolfhir.ErrorWithCode{
			Message:    "Notification focus type is nil",
			StatusCode: http.StatusUnprocessableEntity,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}

	// Add resource metadata to span
	span.SetAttributes(
		attribute.String("fhir.resource_type", *focusReference.Type),
		attribute.String("fhir.resource_reference", *focusReference.Reference),
	)

	log.Ctx(ctx).Info().Msgf("Received notification: Reference %s, Type: %s", *focusReference.Reference, *focusReference.Type)

	if focusReference.Reference == nil {
		err := &coolfhir.ErrorWithCode{
			Message:    "Notification focus reference is nil",
			StatusCode: http.StatusUnprocessableEntity,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	resourceUrl := *focusReference.Reference
	if !strings.HasPrefix(strings.ToLower(resourceUrl), "http:") && !strings.HasPrefix(strings.ToLower(resourceUrl), "https:") {
		err := &coolfhir.ErrorWithCode{
			Message:    "Notification focus.reference is not an absolute URL",
			StatusCode: http.StatusUnprocessableEntity,
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	// TODO: for now, we assume the resource URL is always in the form of <FHIR base url>/<resource type>/<resource id>
	//       Then, we can deduce the FHIR base URL from the resource URL
	fhirBaseURL, _, err := coolfhir.ParseExternalLiteralReference(resourceUrl, *focusReference.Type)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	fhirClient, _, err := s.createFHIRClientForIdentifier(ctx, fhirBaseURL, sender.Organization.Identifier[0])
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	}
	switch *focusReference.Type {
	case "Task":
		var task fhir.Task
		err = fhirClient.Read(*focusReference.Reference, &task)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
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
		err = s.handleTaskNotification(ctx, fhirClient, &task)
		rejection := new(TaskRejection)
		if errors.As(err, &rejection) || errors.As(err, rejection) {
			if err := s.rejectTask(ctx, fhirClient, task, *rejection); err != nil {
				// TODO: what to do here?
				log.Ctx(ctx).Err(err).Msgf("Failed to reject task (id=%s, reason=%s)", *task.Id, rejection.FormatReason())
			}
		} else if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	default:
		log.Ctx(ctx).Debug().Msgf("No handler for notification of type %s, ignoring", *focusReference.Type)
	}

	span.SetAttributes(
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")
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
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"createFHIRClientForExternalRequest",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("operation.name", "CarePlanContributor/CreateFHIRClientForExternalRequest"),
		),
	)
	defer span.End()

	start := time.Now()

	var httpClient *http.Client
	var fhirBaseURL *url.URL
	for _, header := range []string{scpEntityIdentifierHeaderKey, scpFHIRBaseURL} {
		headerValue := request.Header.Get(header)
		if headerValue == "" {
			continue
		}

		span.SetAttributes(
			attribute.String("fhir.header_type", header),
			attribute.String("fhir.header_value", headerValue),
		)

		switch header {
		case scpFHIRBaseURL:
			localCPSURL := tenant.URL(s.orcaPublicURL, careplanservice.FHIRBaseURL)
			var err error
			if headerValue == "local-cps" || headerValue == localCPSURL.String() {
				// Targeted FHIR API is local CPS, either through 'local-cps' or because the target URL matches the local CPS URL
				if !s.cpsEnabled && headerValue == "local-cps" {
					// invalid usage
					err := fmt.Errorf("%s: no local CarePlanService", header)
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, nil, err
				}
				httpClient = s.httpClientForLocalCPS(tenant)
				fhirBaseURL = localCPSURL
				span.SetAttributes(attribute.String("fhir.client_type", "local-cps"))
			} else {
				fhirBaseURL, err = s.parseFHIRBaseURL(headerValue)
				if err != nil {
					err := fmt.Errorf("%s: %w", header, err)
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, nil, err
				}
				_, httpClient, err = s.createFHIRClientForURL(ctx, fhirBaseURL)
				if err != nil {
					err := fmt.Errorf("%s: failed to create HTTP client: %w", header, err)
					span.RecordError(err)
					span.SetStatus(codes.Error, err.Error())
					return nil, nil, err
				}
				span.SetAttributes(attribute.String("fhir.client_type", "external"))
			}
			break
		case scpEntityIdentifierHeaderKey:
			// The header value is in the form of <system>|<value>
			identifier, err := coolfhir.TokenToIdentifier(headerValue)
			if err != nil {
				err := fmt.Errorf("%s: invalid identifier (value=%s): %w", header, headerValue, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, nil, err
			}
			endpoints, err := s.profile.CsdDirectory().LookupEndpoint(ctx, identifier, profile.FHIRBaseURLEndpointName)
			if err != nil {
				err := fmt.Errorf("%s: failed to lookup FHIR base URL (identifier=%s): %w", header, headerValue, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, nil, err
			}
			if len(endpoints) != 1 {
				err := fmt.Errorf("%s: expected one FHIR base URL, got %d (identifier=%s)", header, len(endpoints), headerValue)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, nil, err
			}
			fhirBaseURL, err = s.parseFHIRBaseURL(endpoints[0].Address)
			if err != nil {
				err := fmt.Errorf("%s: registered FHIR base URL is invalid (identifier=%s): %w", header, headerValue, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, nil, err
			}
			_, httpClient, err = s.createFHIRClientForIdentifier(ctx, fhirBaseURL, *identifier)
			if err != nil {
				err := fmt.Errorf("%s: failed to create HTTP client (identifier=%s): %w", header, headerValue, err)
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
				return nil, nil, err
			}
			span.SetAttributes(
				attribute.String("fhir.client_type", "identifier-based"),
				attribute.String("fhir.identifier_system", to.Value(identifier.System)),
			)
			break
		}
	}
	if httpClient == nil || fhirBaseURL == nil {
		err := coolfhir.BadRequest("can't determine the external SCP-node to query from the HTTP request headers")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, nil, err
	}

	span.SetAttributes(
		attribute.String("fhir.base_url", fhirBaseURL.String()),
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")
	return fhirBaseURL, httpClient, nil
}

func (s Service) httpClientForLocalCPS(tenant tenants.Properties) *http.Client {
	httpClient := &http.Client{Transport: internalDispatchHTTPRoundTripper{
		profile: s.profile,
		handler: s.httpHandler,
		requestVisitor: func(request *http.Request) {
			if s.orcaPublicURL.Path == "" || s.orcaPublicURL.Path == "/" {
				return
			}
			originalURL := request.URL
			newURL := new(url.URL)
			*newURL = *request.URL
			// Remove ORCA Public URL prefix from the request path, since we're dispatching internally
			newURL.Path = strings.TrimPrefix(strings.TrimPrefix(originalURL.Path, "/"), strings.TrimPrefix(s.orcaPublicURL.Path, "/"))
			// Add tenant to the request context
			ctx := tenants.WithTenant(request.Context(), tenant)
			// Earlier, I tried http.Request.Clone(), but that caused a redirect-loop. This worked.
			newHTTPRequest, _ := http.NewRequestWithContext(ctx, request.Method, newURL.String(), request.Body)
			newHTTPRequest.Header = request.Header.Clone()
			*request = *newHTTPRequest
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

func (s Service) tenantBasePath(ctx context.Context) (string, error) {
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return "", err
	}
	return basePath + "/" + tenant.ID, nil
}

func createFHIRClient(fhirBaseURL *url.URL, httpClient *http.Client) fhirclient.Client {
	return fhirclient.New(fhirBaseURL, httpClient, coolfhir.Config())
}
