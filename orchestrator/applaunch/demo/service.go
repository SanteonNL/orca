package demo

import (
	"github.com/SanteonNL/orca/orchestrator/careplancontributor"
	"github.com/SanteonNL/orca/orchestrator/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/SanteonNL/orca/orchestrator/util"
	"net/http"
	"net/url"
)

const fhirLauncherKey = "demo"

func init() {
	// Register FHIR client factory that can create FHIR clients when the Demo AppLaunch is used
	coolfhir.ClientFactories[fhirLauncherKey] = func(properties map[string]string) *coolfhir.DefaultFHIRClient {
		fhirServerURL, _ := url.Parse(properties["iss"])
		// Demo AppLaunch connects to backing FHIR server without any authentication,
		// so http.DefaultClient can be used.
		return coolfhir.NewClient(fhirServerURL, http.DefaultClient)
	}
}

func New(sessionManager *user.SessionManager) *Service {
	return &Service{
		sessionManager: sessionManager,
	}
}

type Service struct {
	sessionManager *user.SessionManager
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/demo-app-launch", func(response http.ResponseWriter, request *http.Request) {
		values, ok := util.GetQueryParams(response, request, "patient", "serviceRequest", "practitioner", "iss")
		if !ok {
			return
		}
		s.sessionManager.Create(response, user.SessionData{
			FHIRLauncher: fhirLauncherKey,
			Values:       values,
		})
		http.Redirect(response, request, careplancontributor.LandingURL, http.StatusFound)
	})
}
