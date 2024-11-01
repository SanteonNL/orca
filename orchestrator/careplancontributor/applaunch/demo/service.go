package demo

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"strings"
)

const fhirLauncherKey = "demo"

func init() {
	// Register FHIR client factory that can create FHIR clients when the Demo AppLaunch is used
	clients.Factories[fhirLauncherKey] = func(properties map[string]string) clients.ClientProperties {
		fhirServerURL, _ := url.Parse(properties["iss"])
		return clients.ClientProperties{
			BaseURL: fhirServerURL,
			Client:  http.DefaultTransport,
		}
	}
}

func New(sessionManager *user.SessionManager, config Config, baseURL string, landingUrlPath string) *Service {
	var appLaunchURL string
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		appLaunchURL = baseURL + "/demo-app-launch"
	} else {
		appLaunchURL = "http://localhost" + appLaunchURL + "/demo-app-launch"
	}
	log.Info().Msgf("Demo app launch is (%s)", appLaunchURL)
	return &Service{
		sessionManager: sessionManager,
		config:         config,
		baseURL:        baseURL,
		landingUrlPath: landingUrlPath,
	}
}

type Service struct {
	sessionManager *user.SessionManager
	config         Config
	baseURL        string
	landingUrlPath string
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
	values, ok := getQueryParams(response, request, "patient", "serviceRequest", "practitioner", "iss")
	if !ok {
		return
	}
	s.sessionManager.Create(response, user.SessionData{
		FHIRLauncher: fhirLauncherKey,
		StringValues: values,
	})
	// Redirect to landing page
	targetURL, _ := url.Parse(s.baseURL)
	targetURL = targetURL.JoinPath(s.landingUrlPath)
	http.Redirect(response, request, targetURL.String(), http.StatusFound)
}
