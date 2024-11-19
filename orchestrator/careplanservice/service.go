package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const basePath = "/cps"

// subscriberNotificationTimeout is the timeout for notifying subscribers of changes in FHIR resources.
// We might want to make this configurable at some point.
var subscriberNotificationTimeout = 10 * time.Second

func New(config Config, profile profile.Provider, orcaPublicURL *url.URL) (*Service, error) {
	fhirURL, _ := url.Parse(config.FHIR.BaseURL)
	fhirClientConfig := coolfhir.Config()
	transport, fhirClient, err := coolfhir.NewAuthRoundTripper(config.FHIR, fhirClientConfig)
	if err != nil {
		return nil, err
	}
	s := Service{
		profile:       profile,
		fhirURL:       fhirURL,
		orcaPublicURL: orcaPublicURL,
		transport:     transport,
		fhirClient:    fhirClient,
		subscriptionManager: subscriptions.DerivingManager{
			FhirBaseURL: orcaPublicURL.JoinPath(basePath),
			Channels: subscriptions.CsdChannelFactory{
				Directory:         profile.CsdDirectory(),
				ChannelHttpClient: profile.HttpClient(),
			},
		},
		maxReadBodySize:              fhirClientConfig.MaxResponseSize,
		allowUnmanagedFHIROperations: config.AllowUnmanagedFHIROperations,
	}
	s.handlerProvider = s.defaultHandlerProvider
	s.ensureSearchParameterExists()
	return &s, nil
}

type Service struct {
	orcaPublicURL                *url.URL
	fhirURL                      *url.URL
	transport                    http.RoundTripper
	fhirClient                   fhirclient.Client
	profile                      profile.Provider
	subscriptionManager          subscriptions.Manager
	maxReadBodySize              int
	proxy                        *httputil.ReverseProxy
	allowUnmanagedFHIROperations bool
	handlerProvider              func(method string, resourceType string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)
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
	RequestUrl   *url.URL
	FullUrl      string
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
	if r.FullUrl != "" {
		result.FullUrl = to.Ptr(r.FullUrl)
	}
	return result
}

// FHIRHandlerResult is the result of a FHIRHandler execution.
// It returns:
// - the resource that should be returned, given the transaction result
// - a list of resources that should be notified to subscribers
type FHIRHandlerResult func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error)

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	s.proxy = coolfhir.NewProxy(log.Logger, s.fhirURL, basePath, s.transport)
	baseUrl := s.baseUrl()
	s.profile.RegisterHTTPHandlers(basePath, baseUrl, mux)

	// Binding to actual routing
	mux.HandleFunc("POST "+basePath+"/", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		s.handleBundle(request, httpResponse)
	}))
	// Creating a resource
	mux.HandleFunc("POST "+basePath+"/{type}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleCreateOrUpdate(request, httpResponse, resourceType, "CarePlanService/Create"+resourceType)
	}))
	// Updating a resource by ID
	mux.HandleFunc("PUT "+basePath+"/{type}/{id}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		resourceId := request.PathValue("id")
		s.handleCreateOrUpdate(request, httpResponse, resourceType+"/"+resourceId, "CarePlanService/Update"+resourceType)
	}))
	// Updating a resource by selecting it based on query params
	mux.HandleFunc("PUT "+basePath+"/{type}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleCreateOrUpdate(request, httpResponse, resourceType, "CarePlanService/Update"+resourceType)
	}))
	// Handle reading a specific resource instance
	mux.HandleFunc("GET "+basePath+"/{type}/{id}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		resourceId := request.PathValue("id")
		s.handleGet(request, httpResponse, resourceId, resourceType, "CarePlanService/Get"+resourceType)
	}))
	// Handle search
	mux.HandleFunc("GET "+basePath+"/{type}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		s.handleSearch(request, httpResponse, resourceType, "CarePlanService/Get"+resourceType)
	}))
}

// commitTransaction sends the given transaction Bundle to the FHIR server, and processes the result with the given resultHandlers.
// It returns the result Bundle that should be returned to the client, or an error if the transaction failed.
func (s *Service) commitTransaction(request *http.Request, tx *coolfhir.BundleBuilder, resultHandlers []FHIRHandlerResult) (*fhir.Bundle, error) {
	if log.Trace().Enabled() {
		txJson, _ := json.MarshalIndent(tx, "", "  ")
		log.Trace().Msgf("FHIR Transaction request: %s", txJson)
	}
	var txResult fhir.Bundle
	if err := s.fhirClient.Create(tx.Bundle(), &txResult, fhirclient.AtPath("/")); err != nil {
		log.Error().Err(err).Msgf("Failed to execute transaction (url=%s)", request.URL.String())
		// We don't want to log errors from the backing FHIR server for security reasons.
		return nil, coolfhir.NewErrorWithCode("upstream FHIR server error", http.StatusBadGateway)
	}
	resultBundle := fhir.Bundle{
		Type: fhir.BundleTypeTransactionResponse,
	}
	if log.Trace().Enabled() {
		txJson, _ := json.MarshalIndent(txResult, "", "  ")
		log.Trace().Msgf("FHIR Transaction response: %s", txJson)
	}
	var notificationResources []any
	for entryIdx, resultHandler := range resultHandlers {
		currResult, currNotificationResources, err := resultHandler(&txResult)
		if err != nil {
			return nil, fmt.Errorf("bundle execution succeeded, but couldn't resolve bundle.entry[%d] results: %w", entryIdx, err)
		}
		if currResult != nil {
			resultBundle.Entry = append(resultBundle.Entry, *currResult)
		}
		notificationResources = append(notificationResources, currNotificationResources...)
	}
	resultBundle.Total = to.Ptr(len(resultBundle.Entry))

	for _, notificationResource := range notificationResources {
		s.notifySubscribers(request.Context(), notificationResource)
	}
	return &resultBundle, nil
}

// handleTransactionEntry executes the FHIR operation in the HTTP request. It adds the FHIR operations to be executed to the given transaction Bundle,
// and returns the function that must be executed after the transaction is committed.
func (s *Service) handleTransactionEntry(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	if request.HttpMethod == http.MethodPost {
		// We don't allow creation of resources with a specific ID
		if request.ResourceId != "" {
			return nil, coolfhir.BadRequestError("specifying IDs when creating resources isn't allowed")
		}
	}
	handler := s.handlerProvider(request.HttpMethod, getResourceType(request.ResourcePath))
	if handler == nil {
		return nil, fmt.Errorf("unsupported operation %s %s", request.HttpMethod, request.ResourcePath)
	}
	return handler(ctx, request, tx)
}

func (s *Service) handleUnmanagedOperation(request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.HttpMethod, request.RequestUrl)

	err := s.checkAllowUnmanagedOperations()
	if err != nil {
		return nil, err
	}

	tx.AppendEntry(request.bundleEntry())
	idx := len(tx.Entry) - 1
	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var resultResource []byte
		err := s.fhirClient.Read(*txResult.Entry[idx].Response.Location, &resultResource)
		if err != nil {
			return nil, nil, err
		}
		return &fhir.BundleEntry{
			Resource: resultResource,
			Response: txResult.Entry[idx].Response,
		}, nil, nil
	}, nil
}

func (s *Service) handleCreateOrUpdate(httpRequest *http.Request, httpResponse http.ResponseWriter, resourcePath string, operationName string) {
	tx := coolfhir.Transaction()
	bodyBytes, err := io.ReadAll(httpRequest.Body)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("failed to read request body: %w", err), operationName, httpResponse)
		return
	}
	fhirRequest := FHIRHandlerRequest{
		RequestUrl:   httpRequest.URL,
		HttpMethod:   httpRequest.Method,
		ResourceId:   httpRequest.PathValue("id"),
		ResourcePath: resourcePath,
		ResourceData: bodyBytes,
	}
	result, err := s.handleTransactionEntry(httpRequest.Context(), fhirRequest, tx)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
		return
	}
	txResult, err := s.commitTransaction(httpRequest, tx, []FHIRHandlerResult{result})
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
		return
	}
	if len(txResult.Entry) != 1 {
		log.Logger.Error().Msgf("Expected exactly one entry in transaction result (operation=%s), got %d", operationName, len(txResult.Entry))
		httpResponse.WriteHeader(http.StatusNoContent)
		return
	}
	var statusCode int
	fhirResponse := txResult.Entry[0].Response
	statusParts := strings.Split(fhirResponse.Status, " ")
	if statusCode, err = strconv.Atoi(statusParts[0]); err != nil {
		log.Logger.Warn().Msgf("Failed to parse status code from transaction result (responding with 200 OK): %s", fhirResponse.Status)
		statusCode = http.StatusOK
	}
	coolfhir.SendResponse(httpResponse, statusCode, txResult.Entry[0].Resource, map[string]string{
		// TODO: I won't pretend I tested the other response headers (e.g. Last-Modified or ETag), so we won't set them for now.
		//       Add them (and test them) when needed.
		"Location": *fhirResponse.Location,
	})
}

func (s *Service) handleGet(httpRequest *http.Request, httpResponse http.ResponseWriter, resourceId string, resourceType, operationName string) {
	headers := new(fhirclient.Headers)

	var resource interface{}
	var err error
	switch resourceType {
	case "CarePlan":
		resource, err = s.handleGetCarePlan(httpRequest.Context(), resourceId, headers)
	case "CareTeam":
		resource, err = s.handleGetCareTeam(httpRequest.Context(), resourceId, headers)
	case "Task":
		resource, err = s.handleGetTask(httpRequest.Context(), resourceId, headers)
	case "Patient":
		resource, err = s.handleGetPatient(httpRequest.Context(), resourceId, headers)
	case "Questionnaire":
		resource, err = s.handleGetQuestionnaire(httpRequest.Context(), resourceId, headers)
	case "QuestionnaireResponse":
		resource, err = s.handleGetQuestionnaireResponse(httpRequest.Context(), resourceId, headers)
	case "ServiceRequest":
		resource, err = s.handleGetServiceRequest(httpRequest.Context(), resourceId, headers)
	case "Condition":
		resource, err = s.handleGetCondition(httpRequest.Context(), resourceId, headers)
	default:
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", httpRequest.Method, httpRequest.URL.String())
		err = s.checkAllowUnmanagedOperations()
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
			return
		}
		s.proxy.ServeHTTP(httpResponse, httpRequest)
		return
	}
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
		return
	}
	hdrs := make(map[string]string)
	for key, value := range headers.Header {
		hdrs[key] = value[0]
	}
	coolfhir.SendResponse(httpResponse, http.StatusOK, resource, hdrs)
}

func (s *Service) handleSearch(httpRequest *http.Request, httpResponse http.ResponseWriter, resourceType, operationName string) {
	headers := new(fhirclient.Headers)

	var bundle *fhir.Bundle
	var err error
	switch resourceType {
	case "CarePlan":
		bundle, err = s.handleSearchCarePlan(httpRequest.Context(), httpRequest.URL.Query(), headers)
	case "CareTeam":
		bundle, err = s.handleSearchCareTeam(httpRequest.Context(), httpRequest.URL.Query(), headers)
	case "Task":
		bundle, err = s.handleSearchTask(httpRequest.Context(), httpRequest.URL.Query(), headers)
	case "Patient":
		bundle, err = s.handleSearchPatient(httpRequest.Context(), httpRequest.URL.Query(), headers)
	default:
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", httpRequest.Method, httpRequest.URL.String())
		err = s.checkAllowUnmanagedOperations()
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
			return
		}
		s.proxy.ServeHTTP(httpResponse, httpRequest)
		return
	}
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(err, operationName, httpResponse)
		return
	}
	hdrs := make(map[string]string)
	for key, value := range headers.Header {
		hdrs[key] = value[0]
	}
	coolfhir.SendResponse(httpResponse, http.StatusOK, bundle, hdrs)
}

func (s *Service) handleBundle(httpRequest *http.Request, httpResponse http.ResponseWriter) {
	// Create Bundle
	var bundle fhir.Bundle
	op := "CarePlanService/CreateBundle"
	if err := s.readRequest(httpRequest, &bundle); err != nil {
		coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("invalid Bundle: %w", err), op, httpResponse)
		return
	}
	if bundle.Type != fhir.BundleTypeTransaction {
		coolfhir.WriteOperationOutcomeFromError(errors.New("only bundleType 'Transaction' is supported"), op, httpResponse)
		return
	}
	// Validate: Only allow POST/PUT operations in Bundle
	for _, entry := range bundle.Entry {
		// TODO: Might have to support DELETE in future
		if entry.Request == nil || (entry.Request.Method != fhir.HTTPVerbPOST && entry.Request.Method != fhir.HTTPVerbPUT) {
			coolfhir.WriteOperationOutcomeFromError(errors.New("only write operations are supported in Bundle"), op, httpResponse)
			return
		}
	}
	// Perform each individual operation. Note this doesn't actually create/update resources at the backing FHIR server,
	// but only prepares the transaction.
	tx := coolfhir.Transaction()
	var resultHandlers []FHIRHandlerResult
	for entryIdx, entry := range bundle.Entry {
		// Bundle.entry.request.url must be a relative URL with at most one slash (so Task or Task/1, but not http://example.com/Task or Task/foo/bar)
		if entry.Request.Url == "" {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d].request.url (entry #) is required", entryIdx), op, httpResponse)
			return
		}
		requestUrl, err := url.Parse(entry.Request.Url)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, op, httpResponse)
			return
		}
		if requestUrl.IsAbs() {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d].request.url (entry #) must be a relative URL", entryIdx), op, httpResponse)
			return
		}
		resourcePath := requestUrl.Path
		resourcePathParts := strings.Split(resourcePath, "/")
		if entry.Request == nil || len(resourcePathParts) > 2 {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d].request.url (entry #) has too many paths", entryIdx), op, httpResponse)
			return
		}

		fhirRequest := FHIRHandlerRequest{
			HttpMethod:   entry.Request.Method.Code(),
			RequestUrl:   requestUrl,
			ResourcePath: resourcePath,
			ResourceData: entry.Resource,
		}
		if len(resourcePathParts) == 2 {
			fhirRequest.ResourceId = resourcePathParts[1]
		}
		if entry.FullUrl != nil {
			fhirRequest.FullUrl = *entry.FullUrl
		}
		entryResult, err := s.handleTransactionEntry(httpRequest.Context(), fhirRequest, tx)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d]: %w", entryIdx, err), op, httpResponse)
			return
		}
		resultHandlers = append(resultHandlers, entryResult)
	}
	// Execute the transaction and collect the responses
	resultBundle, err := s.commitTransaction(httpRequest, tx, resultHandlers)
	if err != nil {
		coolfhir.WriteOperationOutcomeFromError(err, "Bundle", httpResponse)
		return
	}

	coolfhir.SendResponse(httpResponse, http.StatusOK, resultBundle)
}

func (s *Service) defaultHandlerProvider(method string, resourcePath string) func(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	switch method {
	case http.MethodPost:
		switch getResourceType(resourcePath) {
		case "Task":
			return s.handleCreateTask
		}
	case http.MethodPut:
		switch getResourceType(resourcePath) {
		case "Task":
			return s.handleUpdateTask
		}
	}
	return func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
		return s.handleUnmanagedOperation(request, tx)
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
	// Send notification for changed resources
	notifyCtx, cancel := context.WithTimeout(ctx, subscriberNotificationTimeout)
	defer cancel()
	if err := s.subscriptionManager.Notify(notifyCtx, resource); err != nil {
		log.Error().Err(err).Msgf("Failed to notify subscribers for %T", resource)
	}
}

func (s Service) baseUrl() *url.URL {
	return s.orcaPublicURL.JoinPath(basePath)
}

func getResourceType(resourcePath string) string {
	return strings.Split(resourcePath, "/")[0]
}

func (s *Service) ensureSearchParameterExists() {
	searchParamID := "CarePlan-subject-identifier"

	// Define the SearchParameter
	searchParam := fhir.SearchParameter{
		Id:          &searchParamID,
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
	}

	// Create a `PreRequestOption` that adds the `If-None-Exist` header to the request
	addHeaderOption := func(client fhirclient.Client, req *http.Request) {
		req.Header.Set("If-None-Exist", fmt.Sprintf("url=%s", searchParam.Url))
	}

	log.Info().Msgf("Performing conditional create on %s", searchParamID)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.fhirClient.CreateWithContext(ctx, &searchParam, &searchParam, addHeaderOption)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to ensure SearchParameter %s", searchParamID)
	} else {
		log.Info().Msgf("Ensured SearchParameter/%s", searchParamID)
	}
}

// checkAllowUnmanagedOperations checks if unmanaged operations are allowed. It errors if they are not.
func (s *Service) checkAllowUnmanagedOperations() error {
	if !s.allowUnmanagedFHIROperations {
		return &coolfhir.ErrorWithCode{
			Message:    "FHIR operation not allowed",
			StatusCode: http.StatusMethodNotAllowed,
		}
	}
	return nil
}
