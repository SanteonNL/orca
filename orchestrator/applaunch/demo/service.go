package demo

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/tinyhttp"
	"github.com/SanteonNL/orca/orchestrator/user"
	"net/http"
	"net/url"
)

const fhirLauncherKey = "demo"

func init() {
	// Register FHIR client factory that can create FHIR clients when the Demo AppLaunch is used
	coolfhir.ClientFactories[fhirLauncherKey] = func(properties map[string]string) *coolfhir.DefaultClient {
		fhirServerURL, _ := url.Parse(properties["iss"])
		// Demo AppLaunch connects to backing FHIR server without any authentication,
		// so http.DefaultClient can be used.
		return coolfhir.NewClient(fhirServerURL, http.DefaultClient)
	}
}

func New(sessionManager *user.SessionManager, config Config, baseURL string) *Service {
	return &Service{
		sessionManager: sessionManager,
		config:         config,
		baseURL:        baseURL,
	}
}

type Service struct {
	sessionManager *user.SessionManager
	config         Config
	baseURL        string
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/demo-app-launch", s.handle)
	//if s.config.FHIRProxyURL != "" {
	//	log.Info().Msgf("Demo FHIR proxy enabled: %s", s.config.FHIRProxyURL)
	//	fhirProxyURL, err := url.Parse(s.config.FHIRProxyURL)
	//	if err != nil {
	//		log.Fatal().Err(err).Msgf("Invalid demo FHIR proxy URL: %s", s.config.FHIRProxyURL)
	//	}
	//	reverseProxy := &httputil.ReverseProxy{
	//		Rewrite: func(r *httputil.ProxyRequest) {
	//			r.SetURL(fhirProxyURL)
	//		},
	//	}
	//	reverseProxy.ErrorHandler = func(responseWriter http.ResponseWriter, request *http.Request, err error) {
	//		responseWriter.Header().Add("Content-Type", "application/fhir+json")
	//		responseWriter.WriteHeader(http.StatusBadGateway)
	//		diagnostics := "The system tried to proxy the FHIR operation, but an error occurred."
	//		data, _ := json.Marshal(fhir.OperationOutcome{
	//			Issue: []fhir.OperationOutcomeIssue{
	//				{
	//					Severity:    fhir.IssueSeverityError,
	//					Diagnostics: &diagnostics,
	//				},
	//			},
	//		})
	//		_, _ = responseWriter.Write(data)
	//	}
	//	mux.HandleFunc("/demo/fhirproxy", reverseProxy.ServeHTTP)
	//}
}

func (s *Service) handle(response http.ResponseWriter, request *http.Request) {
	values, ok := tinyhttp.GetQueryParams(response, request, "patient", "serviceRequest", "practitioner", "iss")
	if !ok {
		return
	}
	s.sessionManager.Create(response, user.SessionData{
		FHIRLauncher: fhirLauncherKey,
		Values:       values,
	})
	// Redirect to landing page
	targetURL, _ := url.Parse(s.baseURL)
	targetURL = targetURL.JoinPath(careplancontributor.LandingURL)
	http.Redirect(response, request, targetURL.String(), http.StatusFound)
}
