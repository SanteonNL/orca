package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplanservice/taskengine"
	"io"
	"net/http"
	"net/url"
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
	return &Service{
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
	}, nil
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
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	proxy := coolfhir.NewProxy(log.Logger, s.fhirURL, basePath, s.transport)
	baseURL := s.orcaPublicURL.JoinPath(basePath)
	s.profile.RegisterHTTPHandlers(basePath, baseURL, mux)
	//
	// Authorized endpoints
	//
	mux.HandleFunc("POST "+basePath+"/Task", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleCreateTask(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, "CarePlanService/CreateTask", writer)
			return
		}
	}))
	mux.HandleFunc("PUT "+basePath+"/Task/{id}", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleUpdateTask(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, "CarePlanService/UpdateTask", writer)
			return
		}
	}))
	mux.HandleFunc("POST "+basePath+"/CarePlan", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleCreateCarePlan(writer, request)
		if err != nil {
			coolfhir.WriteOperationOutcomeFromError(err, "CarePlanService/CreateCarePlan", writer)
			return
		}
	}))
	mux.HandleFunc(basePath+"/*", s.profile.Authenticator(baseURL, func(writer http.ResponseWriter, request *http.Request) {

		if request.Method == http.MethodPost && request.URL.Path == basePath+"/" {
			err := s.handleBundle(writer, request)
			if err != nil {
				// TODO: Adjust operation name on entries in Bundle
				s.writeOperationOutcomeFromError(err, "CarePlanService/CreateCarePlan", writer)
				return
			}
		} else {
			// TODO: Authorize request here
			log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.Method, request.URL.String())
			proxy.ServeHTTP(writer, request)
		}

	}))
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

// convertInto converts the src object into the target object,
// by marshalling src to JSON and then unmarshalling it into target.
func convertInto(src interface{}, target interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
