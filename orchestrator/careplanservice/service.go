package careplanservice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"github.com/rs/zerolog/log"
)

var tokenIntrospectionClient = http.DefaultClient

func New(config Config, nutsPublicURL *url.URL, orcaPublicURL *url.URL, nutsAPIURL *url.URL, ownDID string) (*Service, error) {
	fhirURL, _ := url.Parse(config.FHIR.BaseURL)
	fhirClientConfig := coolfhir.Config()
	transport, fhirClient, err := coolfhir.NewAuthRoundTripper(config.FHIR, fhirClientConfig)
	if err != nil {
		return nil, err
	}
	return &Service{
		fhirURL:         fhirURL,
		orcaPublicURL:   orcaPublicURL,
		nutsPublicURL:   nutsPublicURL,
		nutsAPIURL:      nutsAPIURL,
		ownDID:          ownDID,
		transport:       transport,
		fhirClient:      fhirClient,
		maxReadBodySize: fhirClientConfig.MaxResponseSize,
	}, nil
}

type Service struct {
	orcaPublicURL   *url.URL
	nutsPublicURL   *url.URL
	nutsAPIURL      *url.URL
	ownDID          string
	fhirURL         *url.URL
	transport       http.RoundTripper
	fhirClient      fhirclient.Client
	maxReadBodySize int
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	proxy := coolfhir.NewProxy(log.Logger, s.fhirURL, "/cps", s.transport)
	authConfig := middleware.Config{
		TokenIntrospectionEndpoint: s.nutsAPIURL.JoinPath("internal/auth/v2/accesstoken/introspect").String(),
		TokenIntrospectionClient:   tokenIntrospectionClient,
		BaseURL:                    s.orcaPublicURL.JoinPath("cps"),
	}
	//
	// Unauthorized endpoints
	//
	mux.HandleFunc("GET /cps/.well-known/oauth-protected-resource", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Add("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		md := oauth2.ProtectedResourceMetadata{
			Resource:               s.orcaPublicURL.JoinPath("cps").String(),
			AuthorizationServers:   []string{s.nutsPublicURL.JoinPath("oauth2", s.ownDID).String()},
			BearerMethodsSupported: []string{"header"},
		}
		_ = json.NewEncoder(writer).Encode(md)
	})
	//
	// Authorized endpoints
	//
	mux.HandleFunc("POST /cps/Task", auth.Middleware(authConfig, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleCreateTask(writer, request)
		if err != nil {
			s.writeOperationOutcomeFromError(err, "CarePlanService/CreateTask", writer)
			return
		}
	}))
	mux.HandleFunc("PUT /cps/Task/{id}", auth.Middleware(authConfig, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleUpdateTask(writer, request)
		if err != nil {
			s.writeOperationOutcomeFromError(err, "CarePlanService/UpdateTask", writer)
			return
		}
	}))
	mux.HandleFunc("POST /cps/CarePlan", auth.Middleware(authConfig, func(writer http.ResponseWriter, request *http.Request) {
		err := s.handleCreateCarePlan(writer, request)
		if err != nil {
			s.writeOperationOutcomeFromError(err, "CarePlanService/CreateCarePlan", writer)
			return
		}
	}))
	mux.HandleFunc("/cps/*", auth.Middleware(authConfig, func(writer http.ResponseWriter, request *http.Request) {
		// TODO: Authorize request here
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.Method, request.URL.String())
		proxy.ServeHTTP(writer, request)
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

// convertInto converts the src object into the target object,
// by marshalling src to JSON and then unmarshalling it into target.
func convertInto(src interface{}, target interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
