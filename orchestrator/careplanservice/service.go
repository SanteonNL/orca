package careplanservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplanservice/subscriptions"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
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
		maxReadBodySize:     fhirClientConfig.MaxResponseSize,
		workflows:           taskengine.DefaultWorkflows(),
		questionnaireLoader: taskengine.EmbeddedQuestionnaireLoader{},
	}
	s.handlerProvider = s.defaultHandlerProvider
	return &s, nil
}

type Service struct {
	orcaPublicURL       *url.URL
	fhirURL             *url.URL
	transport           http.RoundTripper
	fhirClient          fhirclient.Client
	profile             profile.Provider
	subscriptionManager subscriptions.Manager
	workflows           taskengine.Workflows
	questionnaireLoader taskengine.QuestionnaireLoader
	maxReadBodySize     int
	proxy               *httputil.ReverseProxy
	handlerProvider     func(method string, resourceType string) func(*http.Request, *coolfhir.TransactionBuilder) (FHIRHandlerResult, error)
}

// FHIRHandler defines a function that handles a FHIR request and returns a function to write the response.
// It may be executed singular, or be part of a Bundle that causes multiple handlers to be executed.
// It is provided with a TransactionBuilder to add FHIR resource operations that must be executed on the backing FHIR server.
// The handler itself must not cause side-effects in the FHIR server: those MUST be effectuated through the transaction.
type FHIRHandler func(http.ResponseWriter, *http.Request, *coolfhir.TransactionBuilder) (FHIRHandlerResult, error)

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
	// Updating a resource
	mux.HandleFunc("PUT "+basePath+"/{type}/{id}", s.profile.Authenticator(baseUrl, func(httpResponse http.ResponseWriter, request *http.Request) {
		resourceType := request.PathValue("type")
		resourceId := request.PathValue("type")
		s.handleCreateOrUpdate(request, httpResponse, resourceType+"/"+resourceId, "CarePlanService/Update"+resourceType)
	}))
	mux.HandleFunc("GET "+basePath+"/{resourcePath...}", s.profile.Authenticator(baseUrl, func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.Method, request.URL.String())
		s.proxy.ServeHTTP(writer, request)
	}))
}

func (s *Service) commitTransaction(request *http.Request, tx *coolfhir.TransactionBuilder, resultHandlers []FHIRHandlerResult) (*fhir.Bundle, error) {
	var txResult fhir.Bundle
	if err := s.fhirClient.Create(tx.Bundle(), &txResult, fhirclient.AtPath("/")); err != nil {
		log.Error().Err(err).Msgf("Failed to execute transaction (url=%s)", request.URL.String())
		// We don't want to log errors from the backing FHIR server for security reasons.
		return nil, coolfhir.NewErrorWithCode("upstream FHIR server error", http.StatusBadGateway)
	}
	var resultBundle fhir.Bundle
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

	for _, notificationResource := range notificationResources {
		s.notifySubscribers(request.Context(), notificationResource)
	}
	return &resultBundle, nil
}

func (s *Service) proceedTransaction(writer http.ResponseWriter, request *http.Request, resourcePath string, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	if request.Method == http.MethodPost {
		// We don't allow creation of resources with a specific ID, so resourceType shouldn't contain any slashes now
		if strings.Contains(resourcePath, "/") {
			return nil, coolfhir.BadRequestError("specifying IDs when creating resources isn't allowed")
		}
	}
	handler := s.handlerProvider(request.Method, getResourceType(resourcePath))
	if handler == nil {
		// TODO: Monitor these, and disallow at a later moment
		log.Warn().Msgf("Unmanaged FHIR Create operation at CarePlanService: %s %s", request.Method, request.URL.String())
		s.proxy.ServeHTTP(writer, request)
		return nil, coolfhir.BadRequestErrorF("unsupported operation: %s %s", request.Method, request.URL.Path)
	}
	return handler(request, tx)
}

func (s *Service) handleCreateOrUpdate(httpRequest *http.Request, httpResponse http.ResponseWriter, resourcePath string, operationName string) {
	tx := coolfhir.Transaction()
	result, err := s.proceedTransaction(httpResponse, httpRequest, resourcePath, tx)
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
	httpResponse.Header().Add("Content-Type", coolfhir.FHIRContentType)
	if fhirResponse.Location != nil {
		httpResponse.Header().Add("Location", *fhirResponse.Location)
	}
	// TODO: I won't pretend I tested the other response headers (e.g. Last-Modified or ETag), so we won't set them for now.
	//       Add them (and test them) when needed.
	httpResponse.WriteHeader(statusCode)
	if err := json.NewEncoder(httpResponse).Encode(txResult.Entry[0].Resource); err != nil {
		log.Logger.Warn().Err(err).Msg("Failed to encode response")
	}
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
		resourcePath := entry.Request.Url
		resourcePathPartsCount := strings.Count(resourcePath, "/") + 1
		if entry.Request == nil || resourcePathPartsCount > 2 {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d].request.url (entry #) must be a relative URL", entryIdx), op, httpResponse)
			return
		}
		entryHttpRequest, err := http.NewRequest(entry.Request.Method.Code(), s.baseUrl().JoinPath(resourcePath).String(), bytes.NewReader(entry.Resource))
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bunde.entry[%d]: unable to dispatch: %w", entryIdx, err), op, httpResponse)
			return
		}
		// If the resource path contains an ID, set it as ID path parameter
		if resourcePathPartsCount == 2 {
			entryHttpRequest.SetPathValue("id", strings.Split(resourcePath, "/")[1])
		}
		entryResult, err := s.proceedTransaction(httpResponse, entryHttpRequest, resourcePath, tx)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(fmt.Errorf("bundle.entry[%d]: %w", entryIdx, err), op, httpResponse)
			return
		}
		resultHandlers = append(resultHandlers, entryResult)
	}
	// Execute the transaction and collect the responses
	resultBundle, err := s.commitTransaction(httpRequest, tx, resultHandlers)
	if err != nil {
		return
	}

	httpResponse.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(httpResponse).Encode(resultBundle); err != nil {
		log.Logger.Error().Err(err).Msg("Failed to encode response")
	}
}

func (s *Service) defaultHandlerProvider(method string, resourcePath string) func(*http.Request, *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	switch method {
	case http.MethodPost:
		switch getResourceType(resourcePath) {
		case "Task":
			return s.handleCreateTask
		case "CarePlan":
			return s.handleCreateCarePlan
		}
	case http.MethodPut:
		switch getResourceType(resourcePath) {
		case "Task":
			return s.handleUpdateTask
		}
	}
	return nil
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

// convertInto converts the src object into the target object,
// by marshalling src to JSON and then unmarshalling it into target.
func convertInto(src interface{}, target interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func getResourceType(resourcePath string) string {
	return strings.Split(resourcePath, "/")[0]
}
