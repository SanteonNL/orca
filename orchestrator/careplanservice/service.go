package careplanservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/webhook"
	"github.com/SanteonNL/orca/orchestrator/events"

	"github.com/SanteonNL/orca/orchestrator/messaging"

	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir/pipeline"

	"github.com/SanteonNL/orca/orchestrator/lib/to"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	lib_otel "github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type FHIRClientFactory func(ctx context.Context) (fhirclient.Client, error)

type FHIROperation interface {
	Handle(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)
}

func TracedHandlerWrapper(operationName string, handler func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error),
) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		tracer := otel.Tracer("orchestrator")
		ctx, span := tracer.Start(
			ctx,
			operationName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("fhir.resource_type", getResourceType(request.ResourcePath)),
				attribute.String("fhir.resource_id", request.ResourceId),
				attribute.String("http.method", request.HttpMethod),
				attribute.String("operation.name", operationName),
			),
		)
		defer span.End()

		if request.Tenant.ID != "" {
			span.SetAttributes(attribute.String("tenant.id", request.Tenant.ID))
		}

		result, err := handler(ctx, request, tx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}

		span.SetStatus(codes.Ok, "")
		return result, nil
	}
}

func FHIRBaseURL(tenantID string, orcaBaseURL *url.URL) *url.URL {
	return orcaBaseURL.JoinPath("cps", tenantID)
}

const basePathWithTenant = basePath + "/{tenant}"
const basePath = "/cps"
const tracerName = "careplanservice"

// subscriberNotificationTimeout is the timeout for notifying subscribers of changes in FHIR resources.
// We might want to make this configurable at some point.
var subscriberNotificationTimeout = 10 * time.Second

func New(config Config, tenantCfg tenants.Config, profile profile.Provider, orcaPublicURL *url.URL, messageBroker messaging.Broker, eventManager events.Manager) (*Service, error) {
	fhirClientConfig := coolfhir.Config()
	tracer := otel.Tracer(tracerName)

	// Initialize connections to per-tenant CPS FHIR servers.
	transportByTenant := make(map[string]http.RoundTripper)
	fhirClientByTenant := make(map[string]fhirclient.Client)
	for _, tenant := range tenantCfg {
		transport, fhirClient, err := coolfhir.NewAuthRoundTripper(tenant.CPS.FHIR, fhirClientConfig)
		if err != nil {
			return nil, err
		}

		transportByTenant[tenant.ID] = coolfhir.NewTracedHTTPTransport(transport, tracer)
		fhirClientByTenant[tenant.ID] = coolfhir.NewTracedFHIRClient(fhirClient, tracer)
		globals.RegisterCPSFHIRClient(tenant.ID, fhirClient)
	}

	subscriptionMgr, err := subscriptions.NewManager(func(tenant tenants.Properties) *url.URL {
		return tenant.URL(orcaPublicURL, FHIRBaseURL)
	}, tenantCfg, subscriptions.CsdChannelFactory{Profile: profile}, messageBroker)
	if err != nil {
		return nil, fmt.Errorf("SubscriptionManager initialization: %w", err)
	}

	s := Service{
		tenants:             tenantCfg,
		profile:             profile,
		orcaPublicURL:       orcaPublicURL,
		transportByTenant:   transportByTenant,
		fhirClientByTenant:  fhirClientByTenant,
		subscriptionManager: subscriptionMgr,
		eventManager:        eventManager,
		maxReadBodySize:     fhirClientConfig.MaxResponseSize,
	}

	// Register event handlers
	for _, handler := range config.Events.WebHooks {
		err := eventManager.Subscribe(CarePlanCreatedEvent{}, webhook.NewEventHandler(handler.URL).Handle)
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to event %T: %w", CarePlanCreatedEvent{}, err)
		}
	}

	s.pipelineByTenant = make(map[string]pipeline.Instance)
	for _, tenant := range tenantCfg {
		cpsBaseURL := tenant.URL(orcaPublicURL, FHIRBaseURL).String()
		s.pipelineByTenant[tenant.ID] = pipeline.New().
			// Rewrite the upstream FHIR server URL in the response body to the public URL of the CPS instance.
			// E.g.: http://fhir-server:8080/fhir -> https://example.com/cps)
			// Required, because Microsoft Azure FHIR doesn't allow overriding the FHIR base URL
			// (https://github.com/microsoft/fhir-server/issues/3526).
			AppendResponseTransformer(pipeline.ResponseBodyRewriter{
				Old: []byte(tenant.CPS.FHIR.BaseURL),
				New: []byte(cpsBaseURL),
			}).
			// Rewrite the upstream FHIR server URL in the response headers (same as for the response body).
			AppendResponseTransformer(pipeline.ResponseHeaderRewriter{
				Old: tenant.CPS.FHIR.BaseURL,
				New: cpsBaseURL,
			})
	}

	s.handlerProvider = s.defaultHandlerProvider
	for _, tenant := range tenantCfg {
		err = s.ensureCustomSearchParametersExists(tenants.WithTenant(context.Background(), tenant))
		if err != nil {
			return nil, err
		}
	}
	return &s, nil
}

type Service struct {
	tenants             tenants.Config
	orcaPublicURL       *url.URL
	transportByTenant   map[string]http.RoundTripper
	fhirClientByTenant  map[string]fhirclient.Client
	pipelineByTenant    map[string]pipeline.Instance
	profile             profile.Provider
	subscriptionManager subscriptions.Manager
	eventManager        events.Manager
	maxReadBodySize     int
	handlerProvider     func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)
}

// FHIRHandler defines a function that handles a FHIR request and returns a function to write the response.
// It may be executed singular, or be part of a Bundle that causes multiple handlers to be executed.
// It is provided with a BundleBuilder to add FHIR resource operations that must be executed on the backing FHIR server.
// The handler itself must not cause side-effects in the FHIR server: those MUST be effectuated through the transaction.
type FHIRHandler func(http.ResponseWriter, *http.Request, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)

type FHIRHandlerRequest struct {
	ResourceId   string
	ResourcePath string
	ResourceData json.RawMessage
	HttpMethod   string
	HttpHeaders  http.Header
	FhirHeaders  *fhirclient.Headers
	QueryParams  url.Values
	RequestUrl   *url.URL
	FullUrl      string
	BaseURL      *url.URL
	Context      context.Context
	Tenant       tenants.Properties
	// Principal contains the identity of the client invoking the FHIR operation.
	Principal *auth.Principal
	// LocalIdentity contains the identifier of the local care organization handling the FHIR operation invocation.
	LocalIdentity *fhir.Identifier
	Upsert        bool
}

func (r FHIRHandlerRequest) bundleEntryWithResource(res any) fhir.BundleEntry {
	result := r.bundleEntry()
	result.Resource, _ = json.Marshal(res)
	return result
}

func (r FHIRHandlerRequest) bundleEntry() fhir.BundleEntry {
	result := fhir.BundleEntry{
		Request: &fhir.BundleEntryRequest{
			Method: coolfhir.HttpMethodToVerb(r.HttpMethod),
			Url:    r.ResourcePath,
		},
		Resource: r.ResourceData,
	}
	if r.RequestUrl != nil {
		query := r.RequestUrl.Query()
		if len(query) > 0 {
			result.Request.Url += "?" + query.Encode()
		}
	}
	coolfhir.WithFullUrl(r.FullUrl)(&result)
	coolfhir.WithRequestHeaders(r.HttpHeaders)(&result)
	return result
}

// FHIRHandlerResult is the result of a FHIRHandler execution.
// It returns:
// - the resources that should be returned, given the transaction result
// - a list of resources that should be notified to subscribers
type FHIRHandlerResult func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error)

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	tracer := otel.Tracer(tracerName)
	// Binding to actual routing
	// Metadata
	mux.HandleFunc("GET "+basePathWithTenant+"/metadata", s.tenants.HttpHandler(func(httpResponse http.ResponseWriter, request *http.Request) {
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
			log.Ctx(request.Context()).Error().Err(err).Msg("Failed to generate CapabilityStatement")
			coolfhir.WriteOperationOutcomeFromError(request.Context(), err, "CarePlanService/Metadata", httpResponse)
			return
		}
		coolfhir.SendResponse(httpResponse, http.StatusOK, md)
	}))
	// Creating a resource
	mux.HandleFunc("POST "+basePathWithTenant+"/{type}", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Create Resource", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleModification(request, httpResponse, resourceType, "CarePlanService/Create"+resourceType)
	}))))
	// Searching for a resource via POST
	mux.HandleFunc("POST "+basePathWithTenant+"/{type}/_search", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Search Resource", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleSearchRequest(request, httpResponse, resourceType, "CarePlanService/Search"+resourceType)
	}))))
	// Handle bundle
	mux.HandleFunc("POST "+basePathWithTenant+"/", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Create Bundle", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		s.handleBundle(request, httpResponse)
	}))))
	mux.HandleFunc("POST "+basePathWithTenant, lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Create Bundle", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		s.handleBundle(request, httpResponse)
	}))))
	// Updating a resource by ID
	mux.HandleFunc("PUT "+basePathWithTenant+"/{type}/{id}", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Update Resource", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		resourceId := request.PathValue("id")
		s.handleModification(request, httpResponse, resourceType+"/"+resourceId, "CarePlanService/Update"+resourceType)
	}))))
	// Updating a resource by selecting it based on query params
	mux.HandleFunc("PUT "+basePathWithTenant+"/{type}", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Update Resource", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleModification(request, httpResponse, resourceType, "CarePlanService/Update"+resourceType)
	}))))
	// Handle reading a specific resource instance
	mux.HandleFunc("GET "+basePathWithTenant+"/{type}/{id}", lib_otel.HandlerWithTracing(tracer, "CarePlanService/FHIR Read Resource", s.tenants.HttpHandler(s.profile.Authenticator(func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		resourceId := request.PathValue("id")
		s.handleGet(request, httpResponse, resourceId, resourceType, "CarePlanService/Get"+resourceType)
	}))))
}

// commitTransaction sends the given transaction Bundle to the FHIR server, and processes the result with the given resultHandlers.
// It returns the result Bundle that should be returned to the client, or an error if the transaction failed.
func (s *Service) commitTransaction(fhirClient fhirclient.Client, request *http.Request, tx *coolfhir.BundleBuilder, resultHandlers []FHIRHandlerResult) (*fhir.Bundle, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		request.Context(),
		"commitTransaction",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.bundle.type", tx.Bundle().Type.String()),
			attribute.Int("fhir.bundle.entry_count", len(tx.Bundle().Entry)),
		),
	)
	defer span.End()
	if log.Trace().Enabled() {
		txJson, _ := json.MarshalIndent(tx, "", "  ")
		log.Ctx(ctx).Trace().Msgf("FHIR Transaction request: %s", txJson)
	}
	var txResult fhir.Bundle
	if err := fhirClient.CreateWithContext(ctx, tx.Bundle(), &txResult, fhirclient.AtPath("/")); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "FHIR transaction failed")
		// If the error is a FHIR OperationOutcome, we should sanitize it before returning it
		txResultJson, _ := json.Marshal(tx.Bundle())
		log.Ctx(ctx).Error().Err(err).
			Msgf("Failed to execute transaction (url=%s): %s", request.URL.String(), string(txResultJson))
		var operationOutcomeErr fhirclient.OperationOutcomeError
		if errors.As(err, &operationOutcomeErr) {
			operationOutcomeErr.OperationOutcome = coolfhir.SanitizeOperationOutcome(operationOutcomeErr.OperationOutcome)
			return nil, operationOutcomeErr
		} else {
			return nil, coolfhir.NewErrorWithCode("upstream FHIR server error", http.StatusBadGateway)
		}
	}
	resultBundle := fhir.Bundle{
		Type: fhir.BundleTypeTransactionResponse,
	}
	if log.Trace().Enabled() {
		txJson, _ := json.MarshalIndent(txResult, "", "  ")
		log.Ctx(ctx).Trace().Msgf("FHIR Transaction response: %s", txJson)
	}
	var notificationResources []any
	for entryIdx, resultHandler := range resultHandlers {
		currResult, currNotificationResources, err := resultHandler(&txResult)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, fmt.Sprintf("result handler failed for entry %d", entryIdx))
			return nil, fmt.Errorf("bundle execution succeeded, but couldn't resolve bundle.entry[%d] results: %w", entryIdx, err)
		}
		for _, entry := range currResult {
			resultBundle.Entry = append(resultBundle.Entry, *entry)
		}
		notificationResources = append(notificationResources, currNotificationResources...)
	}
	resultBundle.Total = to.Ptr(len(resultBundle.Entry))

	for _, notificationResource := range notificationResources {
		s.notifySubscribers(ctx, notificationResource)
	}

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(
		attribute.Int("fhir.transaction.result_entries", len(resultBundle.Entry)),
		attribute.Int("fhir.notification.resources", len(notificationResources)),
	)

	return &resultBundle, nil
}

// handleTransactionEntry executes the FHIR operation in the HTTP request. It adds the FHIR operations to be executed to the given transaction Bundle,
// and returns the function that must be executed after the transaction is committed.
func (s *Service) handleTransactionEntry(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	handler := s.handlerProvider(request.HttpMethod, getResourceType(request.ResourcePath))
	if handler == nil {
		return nil, fmt.Errorf("unsupported operation %s %s", request.HttpMethod, request.ResourcePath)
	}
	return TracedHandlerWrapper("handleTransactionEntry", handler)(ctx, request, tx)
}

func (s *Service) handleUnmanagedOperation(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.HttpMethod, request.RequestUrl.String())

	return nil, fmt.Errorf("unsupported operation %s %s", request.HttpMethod, request.RequestUrl.String())
}

// extractResponseHeadersAndStatus extracts headers and status code from a FHIR response.
// It's a helper function used by both transaction and search response writers.
func (s *Service) extractResponseHeadersAndStatus(entry *fhir.BundleEntry, ctx context.Context) (map[string][]string, int) {
	var statusCode = http.StatusOK
	headers := map[string][]string{}

	if entry == nil || entry.Response == nil {
		return headers, statusCode
	}

	fhirResponse := entry.Response

	// Parse status code from the response
	if fhirResponse.Status != "" {
		statusParts := strings.Split(fhirResponse.Status, " ")
		if parsedCode, err := strconv.Atoi(statusParts[0]); err != nil {
			log.Ctx(ctx).Warn().Msgf("Failed to parse status code from transaction result (responding with 200 OK): %s", fhirResponse.Status)
		} else {
			statusCode = parsedCode
		}
	}

	// Add common headers if present
	if fhirResponse.Location != nil {
		headers["Location"] = []string{*fhirResponse.Location}
	}
	if fhirResponse.Etag != nil {
		headers["ETag"] = []string{*fhirResponse.Etag}
	}
	if fhirResponse.LastModified != nil {
		headers["Last-Modified"] = []string{*fhirResponse.LastModified}
	}

	return headers, statusCode
}

// writeTransactionResponse writes the response from a FHIR transaction to the HTTP response writer.
// It extracts the status code, headers, and resource from the first entry in the transaction result.
func (s *Service) writeTransactionResponse(httpResponse http.ResponseWriter, txResult *fhir.Bundle, ctx context.Context) {
	if len(txResult.Entry) == 0 {
		log.Ctx(ctx).Error().Msg("Expected at least one entry in transaction result, got 0")
		httpResponse.WriteHeader(http.StatusNoContent)
		return
	}

	headers, statusCode := s.extractResponseHeadersAndStatus(&txResult.Entry[0], ctx)

	var resultResource any
	if txResult.Entry[0].Resource != nil {
		resultResource = txResult.Entry[0].Resource
	}

	tenant, _ := tenants.FromContext(ctx)
	s.pipelineByTenant[tenant.ID].
		PrependResponseTransformer(pipeline.ResponseHeaderSetter(headers)).
		DoAndWrite(httpResponse, resultResource, statusCode)
}

// writeSearchResponse writes the response from a FHIR search transaction to the HTTP response writer.
// It returns the entire bundle with all search results.
func (s *Service) writeSearchResponse(httpResponse http.ResponseWriter, txResult *fhir.Bundle, ctx context.Context) {
	if len(txResult.Entry) == 0 {
		log.Ctx(ctx).Warn().Msg("No entries in search result")
		// Return an empty bundle instead of 204 No Content
		tenant, _ := tenants.FromContext(ctx)
		s.pipelineByTenant[tenant.ID].DoAndWrite(httpResponse, &fhir.Bundle{
			Type:  fhir.BundleTypeSearchset,
			Entry: []fhir.BundleEntry{},
			Total: to.Ptr(0),
		}, http.StatusOK)
		return
	}

	// For search results, we get headers from the first entry but return the full bundle
	headers, statusCode := s.extractResponseHeadersAndStatus(&txResult.Entry[0], ctx)
	tenant, _ := tenants.FromContext(ctx)
	s.pipelineByTenant[tenant.ID].
		PrependResponseTransformer(pipeline.ResponseHeaderSetter(headers)).
		DoAndWrite(httpResponse, txResult, statusCode)
}

func (s *Service) handleModification(httpRequest *http.Request, httpResponse http.ResponseWriter, resourcePath string, operationName string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		httpRequest.Context(),
		"handleModification",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", httpRequest.Method),
			attribute.String("fhir.resource_type", getResourceType(resourcePath)),
			attribute.String("operation.name", operationName),
		),
	)
	defer span.End()

	tx := coolfhir.Transaction()
	var bodyBytes []byte
	if httpRequest.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(httpRequest.Body)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(ctx, fmt.Errorf("failed to read request body: %w", err), operationName, httpResponse)
			return
		}
	}

	tenant, err := tenants.FromContext(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, operationName, httpResponse)
		return
	}
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}
	localIdentity, err := s.getLocalIdentity(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	fhirRequest := FHIRHandlerRequest{
		RequestUrl:    httpRequest.URL,
		HttpMethod:    httpRequest.Method,
		HttpHeaders:   coolfhir.FilterRequestHeaders(httpRequest.Header),
		ResourceId:    httpRequest.PathValue("id"),
		ResourcePath:  resourcePath,
		ResourceData:  bodyBytes,
		Context:       ctx,
		Principal:     &principal,
		LocalIdentity: localIdentity,
		Tenant:        tenant,
		BaseURL:       tenant.CPS.FHIR.ParseBaseURL(),
	}
	result, err := s.handleTransactionEntry(ctx, fhirRequest, tx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}
	txResult, err := s.commitTransaction(s.fhirClientByTenant[tenant.ID], httpRequest.WithContext(ctx), tx, []FHIRHandlerResult{result})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	s.writeTransactionResponse(httpResponse, txResult, ctx)

	span.SetStatus(codes.Ok, "")
}

func (s *Service) handleGet(httpRequest *http.Request, httpResponse http.ResponseWriter, resourceId string, resourceType string, operationName string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		httpRequest.Context(),
		"handleGet",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", httpRequest.Method),
			attribute.String("fhir.resource_type", resourceType),
			attribute.String("fhir.resource_id", resourceId),
			attribute.String("operation.name", operationName),
		),
	)
	defer span.End()

	fhirHeaders := new(fhirclient.Headers)

	tenant, err := tenants.FromContext(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, operationName, httpResponse)
		return
	}
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}
	localIdentity, err := s.getLocalIdentity(httpRequest.Context())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	tx := coolfhir.Transaction()
	fhirRequest := FHIRHandlerRequest{
		RequestUrl:    httpRequest.URL,
		HttpMethod:    httpRequest.Method,
		HttpHeaders:   coolfhir.FilterRequestHeaders(httpRequest.Header),
		ResourceId:    resourceId,
		ResourcePath:  resourceType + "/" + resourceId,
		Principal:     &principal,
		LocalIdentity: localIdentity,
		FhirHeaders:   fhirHeaders,
		Tenant:        tenant,
		BaseURL:       tenant.CPS.FHIR.ParseBaseURL(),
		Context:       ctx,
	}

	result, err := s.handleTransactionEntry(ctx, fhirRequest, tx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	txResult, err := s.commitTransaction(s.fhirClientByTenant[tenant.ID], httpRequest.WithContext(ctx), tx, []FHIRHandlerResult{result})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	s.writeTransactionResponse(httpResponse, txResult, ctx)

	span.SetStatus(codes.Ok, "")
}

func (s *Service) handleCreate(resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(resourcePath)

	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {

		var handler func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)

		if resourceType == "Task" {
			handler = s.handleCreateTask
		} else {
			switch resourceType {
			case "ServiceRequest":
				handler = FHIRCreateOperationHandler[*fhir.ServiceRequest]{
					authzPolicy:       CreateServiceRequestAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				}.Handle
			case "Patient":
				handler = FHIRCreateOperationHandler[*fhir.Patient]{
					authzPolicy:       CreatePatientAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,

					validator: &PatientValidator{},
				}.Handle
			case "Questionnaire":
				handler = FHIRCreateOperationHandler[*fhir.Questionnaire]{
					authzPolicy:       CreateQuestionnaireAuthzPolicy(),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				}.Handle
			case "QuestionnaireResponse":
				handler = FHIRCreateOperationHandler[*fhir.QuestionnaireResponse]{
					authzPolicy:       CreateQuestionnaireResponseAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				}.Handle
			case "Condition":
				handler = FHIRCreateOperationHandler[*fhir.Condition]{
					authzPolicy:       CreateConditionAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				}.Handle
			default:
				handler = func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
					return s.handleUnmanagedOperation(ctx, request, tx)
				}
			}
		}
		return TracedHandlerWrapper("handleCreate"+resourceType, handler)(ctx, request, tx)
	}
}

func (s *Service) handleUpdate(resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(resourcePath)

	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {

		var handler func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)

		switch resourceType {
		case "Task":
			handler = s.handleUpdateTask
		case "ServiceRequest":
			handler = FHIRUpdateOperationHandler[*fhir.ServiceRequest]{
				authzPolicy:       UpdateServiceRequestAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
				profile:           s.profile,
				createHandler: &FHIRCreateOperationHandler[*fhir.ServiceRequest]{
					authzPolicy:       CreateServiceRequestAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				},
			}.Handle
		case "Patient":
			handler = FHIRUpdateOperationHandler[*fhir.Patient]{
				authzPolicy:       UpdatePatientAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
				profile:           s.profile,
				createHandler: &FHIRCreateOperationHandler[*fhir.Patient]{
					authzPolicy:       CreatePatientAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				},
			}.Handle
		case "Questionnaire":
			handler = FHIRUpdateOperationHandler[*fhir.Questionnaire]{
				authzPolicy:       UpdateQuestionnaireAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
				profile:           s.profile,
				createHandler: &FHIRCreateOperationHandler[*fhir.Questionnaire]{
					authzPolicy:       CreateQuestionnaireAuthzPolicy(),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				},
			}.Handle
		case "QuestionnaireResponse":
			handler = FHIRUpdateOperationHandler[*fhir.QuestionnaireResponse]{
				authzPolicy:       UpdateQuestionnaireResponseAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
				profile:           s.profile,
				createHandler: &FHIRCreateOperationHandler[*fhir.QuestionnaireResponse]{
					authzPolicy:       CreateQuestionnaireResponseAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				},
			}.Handle
		case "Condition":
			handler = FHIRUpdateOperationHandler[*fhir.Condition]{
				authzPolicy:       UpdateConditionAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
				profile:           s.profile,
				createHandler: &FHIRCreateOperationHandler[*fhir.Condition]{
					authzPolicy:       CreateConditionAuthzPolicy(s.profile),
					fhirClientFactory: s.createFHIRClient,
					profile:           s.profile,
				},
			}.Handle
		default:
			handler = func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
				return s.handleUnmanagedOperation(ctx, request, tx)
			}
		}

		return TracedHandlerWrapper("handleUpdate"+resourceType, handler)(ctx, request, tx)
	}
}

func (s *Service) handleRead(resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(resourcePath)

	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {

		var handleFunc func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error)

		switch resourceType {
		case "Patient":
			handleFunc = FHIRReadOperationHandler[*fhir.Patient]{
				authzPolicy:       ReadPatientAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Condition":
			handleFunc = FHIRReadOperationHandler[*fhir.Condition]{
				authzPolicy:       ReadConditionAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "CarePlan":
			handleFunc = FHIRReadOperationHandler[*fhir.CarePlan]{
				authzPolicy:       ReadCarePlanAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Task":
			handleFunc = FHIRReadOperationHandler[*fhir.Task]{
				authzPolicy:       ReadTaskAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "ServiceRequest":
			handleFunc = FHIRReadOperationHandler[*fhir.ServiceRequest]{
				authzPolicy:       ReadServiceRequestAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Questionnaire":
			handleFunc = FHIRReadOperationHandler[*fhir.Questionnaire]{
				authzPolicy:       ReadQuestionnaireAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "QuestionnaireResponse":
			handleFunc = FHIRReadOperationHandler[*fhir.QuestionnaireResponse]{
				authzPolicy:       ReadQuestionnaireResponseAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		default:
			handleFunc = s.handleUnmanagedOperation
		}

		return TracedHandlerWrapper("handleRead"+resourceType, handleFunc)(ctx, request, tx)
	}
}

func (s *Service) validateSearchRequest(httpRequest *http.Request) error {
	contentType := httpRequest.Header.Get("Content-Type")

	if !strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		return coolfhir.BadRequest("Content-Type must be 'application/x-www-form-urlencoded'")
	}

	// Read the body
	body, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		return coolfhir.BadRequest("Failed to read request body: %w", err)
	}

	bodyString := string(body)

	// Restore the body for later use
	httpRequest.Body = io.NopCloser(bytes.NewBuffer(body))

	if bodyString == "" {
		return nil
	}

	// Custom validation to ensure the encoded body parameters are in the correct format
	split := strings.Split(bodyString, "&")
	for _, param := range split {
		parts := strings.Split(param, "=")
		if len(parts) != 2 {
			return coolfhir.BadRequest("Invalid encoded body parameters")
		}
	}
	return nil
}

func (s *Service) handleSearch(resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(resourcePath)

	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {

		var handleFunc func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error)

		switch resourceType {
		case "Patient":
			handleFunc = FHIRSearchOperationHandler[*fhir.Patient]{
				authzPolicy:       ReadPatientAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Condition":
			handleFunc = FHIRSearchOperationHandler[*fhir.Condition]{
				authzPolicy:       ReadConditionAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "CarePlan":
			handleFunc = FHIRSearchOperationHandler[*fhir.CarePlan]{
				authzPolicy:       ReadCarePlanAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Task":
			handleFunc = FHIRSearchOperationHandler[*fhir.Task]{
				authzPolicy:       ReadTaskAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "ServiceRequest":
			handleFunc = FHIRSearchOperationHandler[*fhir.ServiceRequest]{
				authzPolicy:       ReadServiceRequestAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "Questionnaire":
			handleFunc = FHIRSearchOperationHandler[*fhir.Questionnaire]{
				authzPolicy:       ReadQuestionnaireAuthzPolicy(),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		case "QuestionnaireResponse":
			handleFunc = FHIRSearchOperationHandler[*fhir.QuestionnaireResponse]{
				authzPolicy:       ReadQuestionnaireResponseAuthzPolicy(s.createFHIRClient),
				fhirClientFactory: s.createFHIRClient,
			}.Handle
		default:
			handleFunc = s.handleUnmanagedOperation
		}

		return TracedHandlerWrapper("handleSearch"+resourceType, handleFunc)(ctx, request, tx)
	}
}

func (s *Service) handleSearchRequest(httpRequest *http.Request, httpResponse http.ResponseWriter, resourceType, operationName string) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		httpRequest.Context(),
		"handleSearchRequest",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", httpRequest.Method),
			attribute.String("fhir.resource_type", resourceType),
			attribute.String("operation.name", operationName),
		),
	)
	defer span.End()

	if err := s.validateSearchRequest(httpRequest); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	tenant, err := tenants.FromContext(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, operationName, httpResponse)
		return
	}
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	// Ensure the Content-Type header is set correctly
	contentType := httpRequest.Header.Get("Content-Type")
	if strings.Contains(contentType, ",") {
		contentType = strings.Split(contentType, ",")[0]
		httpRequest.Header.Set("Content-Type", contentType)
	}

	// Parse URL-encoded parameters from the request body
	if err := httpRequest.ParseForm(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}
	queryParams := httpRequest.PostForm

	span.SetAttributes(attribute.Int("fhir.search.param_count", len(queryParams)))

	// Set up the transaction and handler request
	tx := coolfhir.Transaction()

	localIdentity, err := s.getLocalIdentity(httpRequest.Context())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	fhirRequest := FHIRHandlerRequest{
		RequestUrl:    httpRequest.URL,
		HttpMethod:    httpRequest.Method,
		HttpHeaders:   coolfhir.FilterRequestHeaders(httpRequest.Header),
		ResourcePath:  resourceType + "/_search",
		Principal:     &principal,
		LocalIdentity: localIdentity,
		FhirHeaders:   new(fhirclient.Headers),
		QueryParams:   queryParams,
		Tenant:        tenant,
		BaseURL:       tenant.CPS.FHIR.ParseBaseURL(),
	}

	// Get the appropriate search handler
	handler := s.handleSearch(resourceType)

	// Call the handler
	result, err := TracedHandlerWrapper("handleSearch"+resourceType, handler)(ctx, fhirRequest, tx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	// Execute the transaction
	fhirClient, err := s.createFHIRClient(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, operationName, httpResponse)
		return
	}
	txResult, err := s.commitTransaction(fhirClient, httpRequest.WithContext(ctx), tx, []FHIRHandlerResult{result})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, operationName, httpResponse)
		return
	}

	span.SetStatus(codes.Ok, "")
	s.writeSearchResponse(httpResponse, txResult, ctx)
}

func (s *Service) handleBundle(httpRequest *http.Request, httpResponse http.ResponseWriter) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		httpRequest.Context(),
		"handleBundle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", httpRequest.Method),
			attribute.String("operation.name", "CarePlanService/CreateBundle"),
		),
	)
	defer span.End()

	// Create Bundle
	var bundle fhir.Bundle
	op := "CarePlanService/CreateBundle"
	if err := s.readRequest(httpRequest, &bundle); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, coolfhir.BadRequest("invalid Bundle: %w", err), op, httpResponse)
		return
	}

	// Add bundle metadata to span
	span.SetAttributes(
		attribute.String("fhir.bundle.type", bundle.Type.String()),
		attribute.Int("fhir.bundle.entry_count", len(bundle.Entry)),
	)

	if bundle.Type != fhir.BundleTypeTransaction {
		err := coolfhir.BadRequest("only bundleType 'Transaction' is supported")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
		return
	}
	// Validate: Only allow POST/PUT operations in Bundle
	for _, entry := range bundle.Entry {
		if entry.Request == nil || (entry.Request.Method != fhir.HTTPVerbPOST && entry.Request.Method != fhir.HTTPVerbPUT && entry.Request.Method != fhir.HTTPVerbDELETE) {
			err := coolfhir.BadRequest("only write operations are supported in Bundle")
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
			return
		}
	}

	tenant, err := tenants.FromContext(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, op, httpResponse)
		return
	}
	principal, err := auth.PrincipalFromContext(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, op, httpResponse)
		return
	}
	localIdentity, err := s.getLocalIdentity(httpRequest.Context())
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(httpRequest.Context(), err, op, httpResponse)
		return
	}

	// Perform each individual operation. Note this doesn't actually create/update resources at the backing FHIR server,
	// but only prepares the transaction.
	tx := coolfhir.Transaction()
	var resultHandlers []FHIRHandlerResult
	for entryIdx, entry := range bundle.Entry {
		// Bundle.entry.request.url must be a relative URL with at most one slash (so Task or Task/1, but not http://example.com/Task or Task/foo/bar)
		if entry.Request.Url == "" {
			err := coolfhir.BadRequest("bundle.entry[%d].request.url (entry #) is required", entryIdx)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
			return
		}
		requestUrl, err := url.Parse(entry.Request.Url)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
			return
		}
		if requestUrl.IsAbs() {
			err := coolfhir.BadRequest("bundle.entry[%d].request.url (entry #) must be a relative URL", entryIdx)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
			return
		}
		resourcePath := requestUrl.Path
		resourcePathParts := strings.Split(resourcePath, "/")
		if entry.Request == nil || len(resourcePathParts) > 2 {
			err := coolfhir.BadRequest("bundle.entry[%d].request.url (entry #) has too many paths", entryIdx)
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, err, op, httpResponse)
			return
		}
		fhirRequest := FHIRHandlerRequest{
			HttpMethod:    entry.Request.Method.Code(),
			HttpHeaders:   coolfhir.HeadersFromBundleEntryRequest(entry.Request),
			RequestUrl:    requestUrl,
			ResourcePath:  resourcePath,
			ResourceData:  entry.Resource,
			Context:       ctx,
			Principal:     &principal,
			LocalIdentity: localIdentity,
			Tenant:        tenant,
			BaseURL:       tenant.CPS.FHIR.ParseBaseURL(),
		}
		if len(resourcePathParts) == 2 {
			fhirRequest.ResourceId = resourcePathParts[1]
		}
		if entry.FullUrl != nil {
			fhirRequest.FullUrl = *entry.FullUrl
		}
		entryResult, err := s.handleTransactionEntry(ctx, fhirRequest, tx)
		if err != nil {
			var operationOutcomeErr *fhirclient.OperationOutcomeError
			userError := err
			if !errors.As(err, &operationOutcomeErr) {
				userError = coolfhir.BadRequest("bundle.entry[%d]: %w", entryIdx, err)
			}
			span.RecordError(userError)
			span.SetStatus(codes.Error, userError.Error())
			coolfhir.WriteOperationOutcomeFromError(ctx, userError, op, httpResponse)
			return
		}
		resultHandlers = append(resultHandlers, entryResult)
	}
	// Execute the transaction and collect the responses
	resultBundle, err := s.commitTransaction(s.fhirClientByTenant[tenant.ID], httpRequest.WithContext(ctx), tx, resultHandlers)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		coolfhir.WriteOperationOutcomeFromError(ctx, err, "Bundle", httpResponse)
		return
	}
	tenant, _ = tenants.FromContext(httpRequest.Context())
	span.SetAttributes(
		attribute.Int("fhir.bundle.result_entries", len(resultBundle.Entry)),
		attribute.String("tenant.id", tenant.ID),
	)
	span.SetStatus(codes.Ok, "")
	s.pipelineByTenant[tenant.ID].DoAndWrite(httpResponse, resultBundle, http.StatusOK)
}

func (s *Service) defaultHandlerProvider(method string, resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	switch method {
	case http.MethodPost:
		return s.handleCreate(resourcePath)
	case http.MethodPut:
		return s.handleUpdate(resourcePath)
	case http.MethodGet:
		return s.handleRead(resourcePath)
	}
	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		return s.handleUnmanagedOperation(ctx, request, tx)
	}
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

func (s Service) notifySubscribers(ctx context.Context, resource interface{}) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "notify_subscribers",
		trace.WithAttributes(
			attribute.String("notification.resource_type", coolfhir.ResourceType(resource)),
			attribute.Bool("notification.should_notify", shouldNotify(resource)),
		),
	)
	defer span.End()

	if shouldNotify(resource) {
		notifyCtx, cancel := context.WithTimeout(ctx, subscriberNotificationTimeout)
		defer cancel()

		if err := s.subscriptionManager.Notify(notifyCtx, resource); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(attribute.String("notification.status", "failed"))
			log.Ctx(ctx).Error().Err(err).Msgf("Failed to notify subscribers for %T", resource)
		} else {
			span.SetAttributes(
				attribute.String("notification.status", "success"),
			)
			span.SetStatus(codes.Ok, "")
		}
	} else {
		span.SetAttributes(attribute.String("notification.status", "skipped"))
		span.SetStatus(codes.Ok, "skipped - resource type not eligible for notification")
	}
}
func getResourceID(resourcePath string) string {
	parts := strings.Split(resourcePath, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func getResourceType(resourcePath string) string {
	return strings.Split(resourcePath, "/")[0]
}

func searchParameterExists(capabilityStatement fhir.CapabilityStatement, definitionURL string) bool {
	for _, rest := range capabilityStatement.Rest {
		for _, resource := range rest.Resource {
			for _, searchParam := range resource.SearchParam {
				if searchParam.Definition != nil && *searchParam.Definition == definitionURL {
					return true
				}
			}
		}
	}
	return false
}

func (s *Service) ensureCustomSearchParametersExists(ctx context.Context) error {
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return err
	}
	type SearchParam struct {
		SearchParamId string
		SearchParam   fhir.SearchParameter
	}
	params := []SearchParam{
		{
			SearchParamId: "CarePlan-subject-identifier",
			SearchParam: fhir.SearchParameter{
				Id:          to.Ptr("CarePlan-subject-identifier"),
				Url:         "http://zorgbijjou.nl/SearchParameter/CarePlan-subject-identifier",
				Name:        "subject-identifier",
				Status:      fhir.PublicationStatusActive,
				Description: "Search CarePlans by subject identifier",
				Code:        "subject-identifier",
				Base:        []fhir.ResourceType{fhir.ResourceTypeCarePlan},
				Type:        fhir.SearchParamTypeToken,
				Expression:  to.Ptr("CarePlan.subject.identifier"),
				XpathUsage:  to.Ptr(fhir.XPathUsageTypeNormal),
				Xpath:       to.Ptr("f:CarePlan/f:subject/f:identifier"),
				Version:     to.Ptr("1.0"),
				Publisher:   to.Ptr("Zorg Bij Jou"),
				Contact: []fhir.ContactDetail{
					{
						Name: to.Ptr("Support"),
						Telecom: []fhir.ContactPoint{
							{
								System: to.Ptr(fhir.ContactPointSystemEmail),
								Value:  to.Ptr("support@zorgbijjou.nl"),
							},
						},
					},
				},
			},
		},
		{
			SearchParamId: "Task-output-reference",
			SearchParam: fhir.SearchParameter{
				Id:          to.Ptr("Task-output-reference"),
				Url:         "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-output-reference.json",
				Name:        "output-reference",
				Status:      fhir.PublicationStatusActive,
				Description: "Search Tasks by output references and include outputs when searching Tasks",
				Code:        "output-reference",
				Base:        []fhir.ResourceType{fhir.ResourceTypeTask},
				Type:        fhir.SearchParamTypeReference,
				Expression:  to.Ptr("Task.output.value.ofType(Reference)"),
				XpathUsage:  to.Ptr(fhir.XPathUsageTypeNormal),
				Xpath:       to.Ptr("f:Task/f:output/f:valueReference"),
			},
		},
		{
			SearchParamId: "Task-input-reference",
			SearchParam: fhir.SearchParameter{
				Id:          to.Ptr("Task-input-reference"),
				Url:         "http://santeonnl.github.io/shared-care-planning/cps-searchparameter-task-input-reference.json",
				Name:        "input-reference",
				Status:      fhir.PublicationStatusActive,
				Description: "Search Tasks by input references and include inputs when searching Tasks",
				Code:        "input-reference",
				Base:        []fhir.ResourceType{fhir.ResourceTypeTask},
				Type:        fhir.SearchParamTypeReference,
				Expression:  to.Ptr("Task.input.value.ofType(Reference)"),
				XpathUsage:  to.Ptr(fhir.XPathUsageTypeNormal),
				Xpath:       to.Ptr("f:Task/f:input/f:valueReference"),
			},
		},
	}

	fhirClient := s.fhirClientByTenant[tenant.ID]
	var capabilityStatement fhir.CapabilityStatement
	if err := fhirClient.Read("metadata", &capabilityStatement); err != nil {
		return fmt.Errorf("failed to read CapabilityStatement: %w", err)
	}

	reindexURLs := []string{}

	for _, param := range params {
		log.Ctx(ctx).Info().Msgf("Processing custom SearchParameter %s", param.SearchParamId)
		// Check if param exists before creating
		existingParamBundle := fhir.Bundle{}
		err := fhirClient.Search("SearchParameter", url.Values{"url": {param.SearchParam.Url}}, &existingParamBundle)
		if err != nil {
			return fmt.Errorf("search SearchParameter %s: %w", param.SearchParamId, err)
		}

		if len(existingParamBundle.Entry) > 0 {
			log.Ctx(ctx).Info().Msgf("SearchParameter/%s already exists, checking if it needs to be re-indexed", param.SearchParamId)
			// Azure FHIR: if the SearchParameter exists but isn't in the CapabilityStatement, it needs to be re-indexed.
			// See https://learn.microsoft.com/en-us/azure/healthcare-apis/azure-api-for-fhir/how-to-do-custom-search
			if !searchParameterExists(capabilityStatement, param.SearchParam.Url) {
				log.Ctx(ctx).Info().Msgf("SearchParameter/%s needs to be re-indexed", param.SearchParamId)
				reindexURLs = append(reindexURLs, param.SearchParam.Url)
			}
			log.Ctx(ctx).Info().Msgf("SearchParameter/%s already exists, skipping creation", param.SearchParamId)
			continue
		}

		err = fhirClient.CreateWithContext(ctx, param.SearchParam, new(SearchParam))
		if err != nil {
			return fmt.Errorf("create SearchParameter %s: %w", param.SearchParamId, err)
		}
		reindexURLs = append(reindexURLs, param.SearchParam.Url)
		log.Ctx(ctx).Info().Msgf("Created SearchParameter/%s and added to list for batch re-index job.", param.SearchParamId)
	}

	if len(reindexURLs) == 0 {
		log.Ctx(ctx).Info().Msg("No SearchParameters need to be re-indexed")
		return nil
	}

	log.Ctx(ctx).Info().Msgf("Batch reindexing %d SearchParameters", len(reindexURLs))
	reindexParam := fhir.Parameters{
		Parameter: []fhir.ParametersParameter{
			{
				Name:        "targetSearchParameterTypes",
				ValueString: to.Ptr(strings.Join(reindexURLs, ",")),
			},
		},
	}
	var response []byte
	err = fhirClient.CreateWithContext(ctx, reindexParam, &response, fhirclient.AtPath("/$reindex"))
	log.Ctx(ctx).Info().Msgf("Reindexing SearchParameter response %s", string(response))
	if err != nil {
		return fmt.Errorf("batch reindex SearchParameter %s: %w", strings.Join(reindexURLs, ","), err)
	}

	return nil
}

// validateLiteralReferences validates the literal references in the given resource.
// Literal references may be an external URL, but they MUST use HTTPS and be a child of a FHIR base URL
// registered in the CSD. This prevents unsafe external references (e.g. accidentally exchanging resources over HTTP),
// and gives more confidence that the resource can safely be fetched by SCP-nodes.
func validateLiteralReferences(ctx context.Context, prof profile.Provider, resource any) error {
	// Literal references are "reference" fields that contain a string. This can be anywhere in the resource,
	// so we need to recursively search for them.
	resourceAsJson, err := json.Marshal(resource)
	if err != nil {
		// would be very weird
		return err
	}
	resourceAsMap := make(map[string]interface{})
	err = json.Unmarshal(resourceAsJson, &resourceAsMap)
	if err != nil {
		// would be very weird
		return err
	}

	// Make a list of allowed FHIR base URLs, normalize them to all make them end with a slash
	fhirBaseURLs, err := prof.CsdDirectory().LookupEndpoint(ctx, nil, profile.FHIRBaseURLEndpointName)
	if err != nil {
		return fmt.Errorf("unable to list registered FHIR base URLs for validation: %w", err)
	}
	var allowedBaseURLs []string
	for _, fhirBaseURL := range fhirBaseURLs {
		allowedBaseURLs = append(allowedBaseURLs, strings.TrimSuffix(fhirBaseURL.Address, "/")+"/")
	}

	literalReferences := make(map[string]string)
	collectLiteralReferences(resourceAsMap, []string{}, literalReferences)
	for path, reference := range literalReferences {
		lowerCaseRef := strings.ToLower(reference)
		if strings.HasPrefix(lowerCaseRef, "http://") {
			return coolfhir.BadRequest("literal reference is URL with scheme http://, only https:// is allowed (path=%s)", path)
		}
		if strings.HasPrefix(lowerCaseRef, "https://") {
			parsedRef, err := url.Parse(reference)
			if err != nil {
				// weird
				return err
			}
			if slices.Contains(strings.Split(parsedRef.Path, "/"), "..") {
				return coolfhir.BadRequest("literal reference is URL with parent path segment '..' (path=%s)", path)
			}
			if len(parsedRef.Query()) > 0 {
				return coolfhir.BadRequest("literal reference is URL with query parameters")
			}
			// Check if the reference is a child of a registered FHIR base URL
			isRegisteredBaseUrl := false
			for _, allowedBaseURL := range allowedBaseURLs {
				if strings.HasPrefix(parsedRef.String(), allowedBaseURL) {
					isRegisteredBaseUrl = true
					break
				}
			}
			if !isRegisteredBaseUrl {
				return coolfhir.BadRequest("literal reference is not a child of a registered FHIR base URL (path=%s)", path)
			}
		}
	}
	return nil
}

// TODO: This requires some further thought and discussion, implement in the future
// func (s *Service) createErrorAuditEvent(ctx context.Context, err error, action fhir.AuditEventAction, resourceType string, resourceId string) {
// 	principal, localErr := auth.PrincipalFromContext(ctx)
// 	if localErr != nil {
// 		log.Ctx(ctx).Error().Err(localErr).Msg("Failed to get principal for error audit")
// 		return
// 	}

// 	resourceRef := &fhir.Reference{
// 		Reference: to.Ptr(fmt.Sprintf("%s/%s", resourceType, resourceId)),
// 		Type:      to.Ptr(resourceType),
// 	}

// 	auditEvent, localErr := audit.Event(ctx,
// 		action,
// 		resourceRef,
// 		&fhir.Reference{
// 			Identifier: &principal.Organization.Identifier[0],
// 			Type:       to.Ptr("Organization"),
// 		},
// 	)
// 	if localErr != nil {
// 		log.Ctx(ctx).Error().Err(localErr).Msg("Failed to create error audit event")
// 		return
// 	}

// 	// Add error details
// 	auditEvent.Outcome = to.Ptr(fhir.AuditEventOutcome4) // 4 represents minor failure
// 	auditEvent.OutcomeDesc = to.Ptr(err.Error())

// 	if localErr := s.fhirClient.Create(auditEvent, &auditEvent); localErr != nil {
// 		log.Ctx(ctx).Error().Err(localErr).Msg("Failed to store error audit event")
// 	}
// }

func collectLiteralReferences(resource any, path []string, result map[string]string) {
	switch r := resource.(type) {
	case map[string]interface{}:
		for key, value := range r {
			collectLiteralReferences(value, append(path, key), result)
		}
	case []interface{}:
		for i, value := range r {
			collectLiteralReferences(value, append(path, fmt.Sprintf("#%d", i)), result)
		}
	case string:
		if len(path) > 0 && path[len(path)-1] == "reference" {
			// We found a literal reference
			result[strings.Join(path, ".")] = r
		}
	}
}

func (s *Service) getLocalIdentity(ctx context.Context) (*fhir.Identifier, error) {
	localIdentity, err := s.profile.Identities(ctx)
	if err != nil {
		return nil, err
	}
	if len(localIdentity) == 0 || localIdentity[0].Identifier == nil || len(localIdentity[0].Identifier) == 0 {
		return nil, errors.New("no local identity found")
	}
	return &localIdentity[0].Identifier[0], nil
}

func (s *Service) createFHIRClient(ctx context.Context) (fhirclient.Client, error) {
	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	fhirClient, ok := s.fhirClientByTenant[tenant.ID]
	if !ok {
		return nil, fmt.Errorf("FHIR client for tenant %s not found", tenant.ID)
	}
	return fhirClient, nil
}

func shouldNotify(resource any) bool {
	switch coolfhir.ResourceType(resource) {
	case "Task":
		return true
	case "CareTeam":
		return true
	case "CarePlan":
		return true
	default:
		return false
	}
}
