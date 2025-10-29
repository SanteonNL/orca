//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/demo"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/smartonfhir"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/zorgplatform"
	importer "github.com/SanteonNL/orca/orchestrator/careplancontributor/importer"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/op"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/httpserv"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/google/uuid"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/ehr"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/pubsub"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const basePathWithTenant = basePath + "/{tenant}"
const basePath = "/cpc"

var tracer = baseotel.Tracer("careplancontributor")

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
			slog.InfoContext(ctx, "Loading Task Filler Questionnaires/HealthcareService resources from URL", slog.String(logging.FieldUrl, bundleUrl))
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
				slog.InfoContext(ctx, "Synchronizing Task Filler Questionnaires resources to local FHIR store from URLs", slog.Int(logging.FieldCount, len(config.TaskFiller.QuestionnaireSyncURLs)))
				for _, u := range config.TaskFiller.QuestionnaireSyncURLs {
					if err := coolfhir.ImportResources(ctx, questionnaireFhirClient, []string{"Questionnaire", "HealthcareService"}, u); err != nil {
						slog.ErrorContext(
							ctx,
							"Failed to synchronize Task Filler Questionnaire resources",
							slog.String(logging.FieldUrl, u),
							slog.String(logging.FieldResourceType, fhir.ResourceTypeQuestionnaire.String()),
							slog.String(logging.FieldError, err.Error()),
						)
					} else {
						slog.DebugContext(
							ctx,
							"Synchronized Task Filler Questionnaire resources",
							slog.String(logging.FieldUrl, u),
							slog.String(logging.FieldResourceType, fhir.ResourceTypeQuestionnaire.String()),
						)
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
	if config.TaskFiller.TaskAcceptedBundleEndpoint != "" {
		result.notifier, err = ehr.NewNotifier(eventManager, tenants, config.TaskFiller.TaskAcceptedBundleEndpoint, result.createFHIRClientForURL)
		if err != nil {
			return nil, fmt.Errorf("TaskEngine: failed to create EHR notifier: %w", err)
		}
		slog.InfoContext(ctx, "TaskEngine: created EHR notifier", slog.String(logging.FieldEndpoint, config.TaskFiller.TaskAcceptedBundleEndpoint))
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
	var routes []httpserv.Route
	if s.oidcProvider != nil {
		routes = append(routes, []httpserv.Route{
			{
				Path:       basePath + "/login",
				Handler:    s.withSession(s.oidcProvider.HandleLogin),
				Middleware: otel.HandlerWithTracing(tracer, "Login"),
			},
			{
				Path:       basePath + "/",
				Handler:    http.StripPrefix(basePath, s.oidcProvider).ServeHTTP,
				Middleware: otel.HandlerWithTracing(tracer, "OIDCProvider"),
			},
		}...)
	}

	routes = append(routes, []httpserv.Route{
		//
		// The section below defines endpoints specified by Shared Care Planning.
		// These are secured through the profile (e.g. Nuts access tokens)
		//
		// Metadata
		{
			Method:     "GET",
			Path:       basePathWithTenant + "/fhir/metadata",
			Handler:    s.handleFHIRGetMetadata,
			Middleware: httpserv.Chain(s.tenants.HttpHandler),
		},
		// Bundle handling
		{
			Method:  "POST",
			Path:    basePathWithTenant + "/fhir",
			Handler: s.handleFHIRBundle,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProcessBundle"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		{
			Method:  "POST",
			Path:    basePathWithTenant + "/fhir/{$}",
			Handler: s.handleFHIRBundle,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProcessBundle"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		//
		// This is a special endpoint, used by other SCP-nodes to discovery applications.
		//
		{
			Method:  "GET",
			Path:    basePathWithTenant + "/fhir/Endpoint",
			Handler: s.handleFHIRSearchEndpoints,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "DiscoverEndpoints"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		//
		// The following endpoints forward to the FHIR API of the local EHR. They are used by external SCP nodes to query or retrieve resources from the local EHR.
		//
		// The code to GET or POST/_search are the same, so we can use the same Handler for both
		{
			Method:  "GET",
			Path:    basePathWithTenant + "/fhir/{resourceType}/{id}",
			Handler: s.handleFHIRProxyGetOrSearch,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProxyFHIRRead"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		{
			Method:  "POST",
			Path:    basePathWithTenant + "/fhir/{resourceType}/_search",
			Handler: s.handleFHIRProxyGetOrSearch,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProxyFHIRSearch"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		{
			Method:  "GET",
			Path:    basePathWithTenant + "/fhir/{resourceType}",
			Handler: s.handleFHIRProxyGetOrSearch,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProxyFHIRSearch"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		//
		// Custom operations
		//
		{
			Method:  "POST",
			Path:    basePathWithTenant + "/fhir/$import",
			Handler: s.handleFHIRImportOperation,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ImportOperation"),
				s.tenants.HttpHandler,
				s.profile.Authenticator,
			),
		},
		//
		// The section below defines endpoints used for integrating the local EHR with ORCA.
		// They are NOT specified by SCP. Authorization is specific to the local EHR.
		//
		// This endpoint is used by the EHR and ORCA Frontend to query the FHIR API of a remote SCP-node.
		// The remote SCP-node to query can be specified using the following HTTP headers:
		// - X-Scp-Entity-Identifier: Uses the identifier of the SCP-node to query (in the form of <system>|<value>), to resolve the registered FHIR base URL
		{
			Path:    basePathWithTenant + "/external/fhir/{rest...}",
			Handler: s.handleFHIRExternalProxy,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProxyExternalFHIR"),
				s.tenants.HttpHandler,
				s.withUserAuth,
			),
		},
		{
			Method:  "GET",
			Path:    basePath + "/context",
			Handler: s.withSession(s.handleGetContext),
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "GetContext"),
				s.withUserAuth,
			),
		},
		{
			Method:  "GET",
			Path:    basePathWithTenant + "/ehr/fhir/{rest...}",
			Handler: s.withSession(s.handleProxyAppRequestToEHR),
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "ProxyAppToEHR"),
				s.tenants.HttpHandler,
				s.withUserAuth,
			),
		},
		{
			Path:    "/logout",
			Handler: s.handleLogout,
			Middleware: httpserv.Chain(
				otel.HandlerWithTracing(tracer, "Logout"),
				s.withUserAuth,
			),
		},
	}...)

	httpserv.RegisterRoutes(mux, routes...)

	// App launch endpoints
	for _, appLaunch := range s.appLaunches {
		appLaunch.RegisterHandlers(mux)
	}
}

func (s *Service) initializeAppLaunches(sessionManager *user.SessionManager[session.Data], strictMode bool) error {
	frontendUrl, _ := url.Parse(s.config.FrontendConfig.URL)

	if s.config.AppLaunch.SmartOnFhir.Enabled {
		service, err := smartonfhir.New(s.config.AppLaunch.SmartOnFhir, s.tenants, sessionManager, s.orcaPublicURL, frontendUrl, strictMode)
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
			s.ehrFHIRClientByTenant[tenantID] = coolfhir.NewTracedFHIRClient(fhirClient, tracer)
		}
	}
	return nil
}

// withSession is a middleware that retrieves the session for the given request.
// It then calls the given handler function and provides the session.
// If there's no active session, it returns a 401 Unauthorized response.
func (s Service) withSession(next func(response http.ResponseWriter, request *http.Request, session *session.Data)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		sessionData, err := s.getAndValidateUserSession(request)
		if err != nil {
			// Invalid session/request
			http.Error(response, err.Error(), http.StatusForbidden)
			return
		}
		if sessionData == nil {
			http.Error(response, "no session found", http.StatusUnauthorized)
			return
		}
		next(response, request, sessionData)
	}
}

// handleFHIRGetMetadata handles the FHIR CapabilityStatement request.
func (s *Service) handleFHIRGetMetadata(httpResponse http.ResponseWriter, request *http.Request) {
	md := fhir.CapabilityStatement{
		FhirVersion: fhir.FHIRVersion4_0_1,
		Date:        time.Now().Format(time.RFC3339),
		Status:      fhir.PublicationStatusActive,
		Kind:        fhir.CapabilityStatementKindInstance,
		Format:      []string{"json"},
		Rest: []fhir.CapabilityStatementRest{
			{
				Mode: fhir.RestfulCapabilityModeServer,
			},
		},
	}
	if err := s.profile.CapabilityStatement(request.Context(), &md); err != nil {
		slog.ErrorContext(request.Context(), "Failed to generate CapabilityStatement", slog.String(logging.FieldError, err.Error()))
		coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanContributor/Metadata", httpResponse)
		return
	}
	coolfhir.SendResponse(httpResponse, http.StatusOK, md)
}

// handleFHIRBundle handles a FHIR Bundle request, which can be either a subscription notification or a batch request.
func (s *Service) handleFHIRBundle(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	var bundle fhir.Bundle
	if err := json.NewDecoder(httpRequest.Body).Decode(&bundle); err != nil {
		err := coolfhir.BadRequest("failed to decode bundle: %w", err)
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, "CarePlanContributor/CreateBundle", httpResponse)
		return
	}
	if coolfhir.IsSubscriptionNotification(&bundle) {
		if err := s.handleNotification(httpRequest.Context(), (*coolfhir.SubscriptionNotification)(&bundle)); err != nil {
			coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, "CarePlanContributor/CreateBundle", httpResponse)
			return
		}
		coolfhir.SendResponse(httpResponse, http.StatusOK, &fhir.Bundle{Type: fhir.BundleTypeHistory})
		return
	} else if bundle.Type == fhir.BundleTypeBatch {
		result, err := s.handleFHIRBatchBundle(httpRequest, bundle)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, "CarePlanContributor/CreateBundle", httpResponse)
			return
		}
		coolfhir.SendResponse(httpResponse, http.StatusOK, result)
		return
	}
	err := coolfhir.BadRequest("bundle type not supported: %s", bundle.Type.String())
	coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, "CarePlanContributor/CreateBundle", httpResponse)
}

func (s *Service) handleFHIRSearchEndpoints(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	if len(httpRequest.URL.Query()) > 0 {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), coolfhir.BadRequest("search parameters are not supported on this endpoint"), fmt.Sprintf("CarePlanContributor/%s %s", httpRequest.Method, httpRequest.URL.Path), httpResponse)
		return
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
	coolfhir.SendResponse(httpResponse, http.StatusOK, bundle, nil)
}

func (s *Service) handleFHIRProxyGetOrSearch(writer http.ResponseWriter, request *http.Request) {
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
		slog.ErrorContext(
			request.Context(),
			"FHIR request from external CPC to local EHR failed",
			slog.String(logging.FieldError, err.Error()),
			slog.String(logging.FieldUrl, request.URL.String()),
		)
		// If the error is a FHIR OperationOutcome, we should sanitize it before returning it
		var operationOutcomeErr fhirclient.OperationOutcomeError
		if errors.As(err, &operationOutcomeErr) {
			operationOutcomeErr.OperationOutcome = coolfhir.SanitizeOperationOutcome(operationOutcomeErr.OperationOutcome)
			err = operationOutcomeErr
		}
		coolfhir.WriteOperationOutcomeFromError(request.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", request.Method, request.URL.Path), writer)
		return
	}
}

// handleProxyAppRequestToEHR handles a request from the CPC application (e.g. Frontend), forwarding it to the local EHR's FHIR API.
func (s Service) handleProxyAppRequestToEHR(writer http.ResponseWriter, request *http.Request, sessionData *session.Data) {
	ctx, span := tracer.Start(
		request.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.HTTPMethod, request.Method),
			attribute.String(otel.HTTPURL, request.URL.String()),
		),
	)
	defer span.End()

	clientFactory := clients.Factories[sessionData.FHIRLauncher](sessionData.LauncherProperties)
	proxyBasePath, err := s.tenantBasePath(ctx)
	if err != nil {
		otel.Error(span, err)
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
		attribute.String("fhir.resource_path", resourcePath),
	)
	span.SetStatus(codes.Ok, "")
}

// handleProxyExternalRequestToEHR handles a request from an external SCP-node (e.g. CarePlanContributor), forwarding it to the local EHR's FHIR API.
// This is typically used by remote parties to retrieve patient data from the local EHR.
func (s Service) handleProxyExternalRequestToEHR(writer http.ResponseWriter, request *http.Request) error {
	ctx, span := tracer.Start(
		request.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.HTTPMethod, request.Method),
			attribute.String(otel.HTTPURL, request.URL.String()),
		),
	)
	defer span.End()

	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return err
	}

	ehrProxy := s.ehrFHIRProxyByTenant[tenant.ID]
	if ehrProxy == nil {
		return otel.Error(span, coolfhir.BadRequest("EHR API is not supported"))
	}

	slog.DebugContext(ctx, "Handling external FHIR API request")
	_, err = s.authorizeScpMember(request.WithContext(ctx))
	if err != nil {
		return otel.Error(span, err)
	}

	ehrProxy.ServeHTTP(writer, request.WithContext(ctx))

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

func (s Service) getAndValidateUserSession(request *http.Request) (*session.Data, error) {
	// Determine if the request is scoped to a tenant (/cpc/{tenant}/...).
	var tenant *tenants.Properties
	if request.PathValue("tenant") != "" {
		tenantValue, err := tenants.FromContext(request.Context())
		if err != nil {
			return nil, errors.New("failed to determine tenant from request")
		}
		tenant = &tenantValue
	}

	// Get session, and if there is one, make sure it matches the tenant (if any)
	if data := s.SessionManager.Get(request); data != nil {
		if tenant != nil && data.TenantID != tenant.ID {
			return nil, errors.New("session tenant does not match request tenant")
		}
		return data, nil
	}
	// No user session found
	return nil, nil
}

func (s Service) withUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		sessionData, err := s.getAndValidateUserSession(request)
		if err != nil {
			// Invalid session/request
			http.Error(response, err.Error(), http.StatusForbidden)
			return
		}
		if sessionData != nil {
			// Valid user session found, proceed
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
					slog.ErrorContext(request.Context(), "Failed to validate ADB2C token", slog.String(logging.FieldError, err.Error()))
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
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(),
	)
	defer span.End()

	sender, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return otel.Error(span, err)
	}

	notification, ok := resource.(*coolfhir.SubscriptionNotification)
	if !ok {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "failed to cast resource to notification",
			StatusCode: http.StatusBadRequest,
		})
	}
	if notification == nil {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "notification is nil",
			StatusCode: http.StatusInternalServerError,
		})
	}

	focusReference, err := notification.GetFocus()
	if err != nil {
		return otel.Error(span, err)
	}
	if focusReference == nil {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "Notification focus not found",
			StatusCode: http.StatusUnprocessableEntity,
		})
	}

	if focusReference.Type == nil {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "Notification focus type is nil",
			StatusCode: http.StatusUnprocessableEntity,
		})
	}

	// Add resource metadata to span
	span.SetAttributes(
		attribute.String(otel.FHIRResourceType, *focusReference.Type),
		attribute.String("fhir.resource_reference", *focusReference.Reference),
	)

	slog.InfoContext(
		ctx,
		"Received notification",
		slog.String(logging.FieldResourceReference, *focusReference.Reference),
		slog.String(logging.FieldResourceType, *focusReference.Type),
	)

	if focusReference.Reference == nil {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "Notification focus reference is nil",
			StatusCode: http.StatusUnprocessableEntity,
		})
	}
	resourceUrl := *focusReference.Reference
	if !strings.HasPrefix(strings.ToLower(resourceUrl), "http:") && !strings.HasPrefix(strings.ToLower(resourceUrl), "https:") {
		return otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    "Notification focus.reference is not an absolute URL",
			StatusCode: http.StatusUnprocessableEntity,
		})
	}
	// TODO: for now, we assume the resource URL is always in the form of <FHIR base url>/<resource type>/<resource id>
	//       Then, we can deduce the FHIR base URL from the resource URL
	fhirBaseURL, _, err := coolfhir.ParseExternalLiteralReference(resourceUrl, *focusReference.Type)
	if err != nil {
		return otel.Error(span, err)
	}
	fhirClient, _, err := s.createFHIRClientForIdentifier(ctx, fhirBaseURL, sender.Organization.Identifier[0])
	if err != nil {
		return otel.Error(span, err)
	}
	switch *focusReference.Type {
	case "Task":
		var task fhir.Task
		err = fhirClient.Read(*focusReference.Reference, &task)
		if err != nil {
			return otel.Error(span, err)
		}
		//insert the meta.source - can be used to determine the X-Scp-Context
		if task.Meta == nil {
			task.Meta = &fhir.Meta{}
		}

		if task.Meta.Source != nil && *task.Meta.Source != resourceUrl {
			slog.WarnContext(
				ctx,
				"Task already has a source, overwriting",
				slog.String("id", *task.Id),
				slog.String("source_original", *task.Meta.Source),
				slog.String("source_new", resourceUrl),
			)
		}

		task.Meta.Source = &resourceUrl
		err = s.handleTaskNotification(ctx, fhirClient, &task)
		rejection := new(TaskRejection)
		if errors.As(err, &rejection) || errors.As(err, rejection) {
			if err := s.rejectTask(ctx, fhirClient, task, *rejection); err != nil {
				// TODO: what to do here?
				slog.ErrorContext(
					ctx,
					"Failed to reject task",
					slog.String("id", *task.Id),
					slog.String("reason", rejection.FormatReason()),
				)
			}
		} else if err != nil {
			return otel.Error(span, err)
		}
	default:
		slog.DebugContext(ctx, "No handler for notification of type, ignoring", slog.String(logging.FieldResourceType, *focusReference.Type))
	}

	span.SetAttributes()
	span.SetStatus(codes.Ok, "")
	return nil
}

func (s Service) rejectTask(ctx context.Context, client fhirclient.Client, task fhir.Task, rejection TaskRejection) error {
	slog.InfoContext(
		ctx,
		"Rejecting task",
		slog.String("id", *task.Id),
		slog.String("reason", rejection.FormatReason()),
	)
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
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

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
			// Check if the user is authenticated by session i.e. request is from the Frontend, if so, only allow local-cps as header value
			checkSession, err := s.getAndValidateUserSession(request)
			if err != nil {
				return nil, nil, otel.Error(span, err)
			}

			localCPSURL := tenant.URL(s.orcaPublicURL, careplanservice.FHIRBaseURL)
			if checkSession != nil && headerValue != "local-cps" && headerValue != localCPSURL.String() {
				return nil, nil, otel.Error(span, coolfhir.BadRequest("%s: only 'local-cps' or local CPS URL is allowed when authenticated by user session", header))
			}

			if headerValue == "local-cps" || headerValue == localCPSURL.String() {
				// Targeted FHIR API is local CPS, either through 'local-cps' or because the target URL matches the local CPS URL
				if !s.cpsEnabled && headerValue == "local-cps" {
					// invalid usage
					return nil, nil, otel.Error(span, fmt.Errorf("%s: no local CarePlanService", header))
				}
				httpClient = s.httpClientForLocalCPS(tenant)
				fhirBaseURL = localCPSURL
				span.SetAttributes(attribute.String(otel.FHIRClientType, "local-cps"))
			} else {
				fhirBaseURL, err = s.parseFHIRBaseURL(headerValue)
				if err != nil {
					return nil, nil, otel.Error(span, fmt.Errorf("%s: %w", header, err))
				}
				_, httpClient, err = s.createFHIRClientForURL(ctx, fhirBaseURL)
				if err != nil {
					return nil, nil, otel.Error(span, fmt.Errorf("%s: failed to create HTTP client: %w", header, err))
				}
				span.SetAttributes(attribute.String(otel.FHIRClientType, "external"))
			}
			break
		case scpEntityIdentifierHeaderKey:
			// The header value is in the form of <system>|<value>
			identifier, err := coolfhir.TokenToIdentifier(headerValue)
			if err != nil {
				return nil, nil, otel.Error(span, fmt.Errorf("%s: invalid identifier (value=%s): %w", header, headerValue, err))
			}
			endpoints, err := s.profile.CsdDirectory().LookupEndpoint(ctx, identifier, profile.FHIRBaseURLEndpointName)
			if err != nil {
				return nil, nil, otel.Error(span, fmt.Errorf("%s: failed to lookup FHIR base URL (identifier=%s): %w", header, headerValue, err))
			}
			if len(endpoints) != 1 {
				return nil, nil, otel.Error(span, fmt.Errorf("%s: expected one FHIR base URL, got %d (identifier=%s)", header, len(endpoints), headerValue))
			}
			fhirBaseURL, err = s.parseFHIRBaseURL(endpoints[0].Address)
			if err != nil {
				return nil, nil, otel.Error(span, fmt.Errorf("%s: registered FHIR base URL is invalid (identifier=%s): %w", header, headerValue, err))
			}
			_, httpClient, err = s.createFHIRClientForIdentifier(ctx, fhirBaseURL, *identifier)
			if err != nil {
				return nil, nil, otel.Error(span, fmt.Errorf("%s: failed to create HTTP client (identifier=%s): %w", header, headerValue, err))
			}
			span.SetAttributes(
				attribute.String(otel.FHIRClientType, "identifier-based"),
				attribute.String("fhir.identifier_system", to.Value(identifier.System)),
			)
			break
		}
	}
	if httpClient == nil || fhirBaseURL == nil {
		return nil, nil, otel.Error(span, coolfhir.BadRequest("can't determine the external SCP-node to query from the HTTP request headers"))
	}

	span.SetAttributes(
		attribute.String(otel.FHIRBaseURL, fhirBaseURL.String()),
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

func (s *Service) handleFHIRImportOperation(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	result, err := s.handleImport(httpRequest)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, "CarePlanContributor/Import", httpResponse)
		return
	}
	coolfhir.SendResponse(httpResponse, http.StatusOK, result, nil)
}

func (s *Service) handleFHIRExternalProxy(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	// TODO: Extract relevant data from the bearer JWT
	fhirBaseURL, httpClient, err := s.createFHIRClientForExternalRequest(httpRequest.Context(), httpRequest)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", httpRequest.Method, httpRequest.URL.Path), httpResponse)
		return
	}
	proxyBasePath, err := s.tenantBasePath(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, fmt.Sprintf("CarePlanContributor/%s %s", httpRequest.Method, httpRequest.URL.Path), httpResponse)
		return
	}
	proxyBasePath += "/external/fhir/"
	fhirProxy := coolfhir.NewProxy("EHR(local)->EHR(external) FHIR proxy", fhirBaseURL, proxyBasePath, s.orcaPublicURL.JoinPath(proxyBasePath), coolfhir.NewTracedHTTPTransport(httpClient.Transport, tracer), true, true)
	fhirProxy.ServeHTTP(httpResponse, httpRequest)
}

func (s Service) handleImport(httpRequest *http.Request) (*fhir.Bundle, error) {
	ctx, span := tracer.Start(
		httpRequest.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return nil, otel.Error(span, err)
	}
	if !tenant.EnableImport {
		return nil, otel.Error(span, coolfhir.NewErrorWithCode("import is not enabled for this tenant", http.StatusForbidden))
	}
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, otel.Error(span, err)
	}

	//
	// Parse input parameters
	//
	requestBody, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	var params fhir.Parameters
	if err := json.Unmarshal(requestBody, &params); err != nil {
		return nil, fmt.Errorf("failed to parse request body as FHIR Parameters: %w", err)
	}
	patientIdentifier, err := getIdentifierParameter(params, "patient")
	if err != nil {
		return nil, otel.Error(span, err)
	}
	workflowID, err := getIdentifierParameter(params, "chipsoft_zorgplatform_workflowid")
	if err != nil && !strings.HasPrefix(err.Error(), "missing parameter") {
		return nil, otel.Error(span, err)
	}
	serviceRequest, err := getCodingParameter(params, "servicerequest")
	if err != nil {
		return nil, otel.Error(span, err)
	}
	condition, err := getCodingParameter(params, "condition")
	if err != nil {
		return nil, otel.Error(span, err)
	}
	startDate, err := getParameter[time.Time](params, "start", func(parameter fhir.ParametersParameter) *time.Time {
		if parameter.ValueDateTime == nil {
			return nil
		}
		result, err := time.Parse(time.RFC3339, *parameter.ValueDateTime)
		if err != nil {
			return nil
		}
		return &result
	})
	if err != nil {
		return nil, otel.Error(span, err)
	}

	// Read patient information from EHR or Zorgplatform
	var patient fhir.Patient
	var patientBundle fhir.Bundle
	ehrFHIRClient := s.ehrFHIRClientByTenant[tenant.ID]
	var externalIdentifier fhir.Identifier
	if workflowID != nil {
		// Zorgplatform
		reqHeadersOpts := fhirclient.RequestHeaders(map[string][]string{
			"X-Scp-PatientID":  {*patientIdentifier.Value},
			"X-Scp-WorkflowID": {*workflowID.Value},
		})
		// Fetch patient from EHR according to BgZ (the general-practitioner is not needed, but added for conformance).
		if err = ehrFHIRClient.SearchWithContext(ctx, "Patient", url.Values{"_include": []string{"Patient:general-practitioner"}}, &patientBundle, reqHeadersOpts); err != nil {
			return nil, otel.Error(span, fmt.Errorf("unable to fetch Patient and Practitioner bundle: %w", err))
		}
		externalIdentifier = *workflowID
	} else {
		// Demo EHR
		if err = ehrFHIRClient.SearchWithContext(ctx, "Patient", url.Values{"identifier": []string{coolfhir.IdentifierToToken(*patientIdentifier)}}, &patientBundle); err != nil {
			return nil, otel.Error(span, fmt.Errorf("unable to fetch Patient and Practitioner bundle: %w", err))
		}
		externalIdentifier = fhir.Identifier{
			System: to.Ptr("urn:ietf:rfc:4122"),
			Value:  to.Ptr(uuid.New().String()),
		}
	}
	if err := coolfhir.ResourceInBundle(&patientBundle, coolfhir.EntryIsOfType("Patient"), &patient); err != nil {
		return nil, otel.Error(span, fmt.Errorf("unable to find Patient resource in Bundle: %w", err))
	}

	taskRequesterCandidates, err := s.profile.Identities(ctx)
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("unable to determine Task.requester: %w", err))
	}
	if len(taskRequesterCandidates) != 1 {
		return nil, otel.Error(span, fmt.Errorf("unable to determine Task.requester: found %d candidates", len(taskRequesterCandidates)))
	}
	taskRequester := taskRequesterCandidates[0]

	slog.InfoContext(
		ctx,
		"Invoking CPS $import operation",
	)
	cpsFHIRClient := fhirclient.New(tenant.URL(s.orcaPublicURL, careplanservice.FHIRBaseURL), s.httpClientForLocalCPS(tenant), coolfhir.Config())
	result, err := importer.Import(ctx, cpsFHIRClient, taskRequester, principal.Organization, *patientIdentifier, patient, externalIdentifier, *serviceRequest, *condition, *startDate)
	if err != nil {
		return nil, otel.Error(span, fmt.Errorf("import failed: %w", err))
	}
	return result, nil
}

func (s *Service) handleLogout(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	s.SessionManager.Destroy(httpResponse, httpRequest)
	// If there is a 'Referer' value in the header, redirect to that URL
	if referer := httpRequest.Header.Get("Referer"); referer != "" {
		http.Redirect(httpResponse, httpRequest, referer, http.StatusFound)
	} else {
		// This redirection will be handled by middleware in the frontend
		http.Redirect(httpResponse, httpRequest, s.config.FrontendConfig.URL, http.StatusOK)
	}
}

func getIdentifierParameter(params fhir.Parameters, name string) (*fhir.Identifier, error) {
	return getParameter[fhir.Identifier](params, name, func(parameter fhir.ParametersParameter) *fhir.Identifier {
		return parameter.ValueIdentifier
	})
}

func getCodingParameter(params fhir.Parameters, name string) (*fhir.Coding, error) {
	return getParameter[fhir.Coding](params, name, func(parameter fhir.ParametersParameter) *fhir.Coding {
		return parameter.ValueCoding
	})
}

func getParameter[T any](params fhir.Parameters, name string, getter func(fhir.ParametersParameter) *T) (*T, error) {
	for _, param := range params.Parameter {
		if name == param.Name {
			value := getter(param)
			if value == nil {
				return nil, fmt.Errorf("parameter %s has no or an invalid value (expected %T)", name, *new(T))
			}
			return value, nil
		}
	}
	return nil, fmt.Errorf("missing parameter %s", name)
}

func createFHIRClient(fhirBaseURL *url.URL, httpClient *http.Client) fhirclient.Client {
	return fhirclient.New(fhirBaseURL, httpClient, coolfhir.Config())
}
