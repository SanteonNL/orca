package careplancontributor

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/assets"
	"github.com/SanteonNL/orca/orchestrator/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

const LandingURL = "/contrib/"

func New(sessionManager *user.SessionManager) *Service {
	return &Service{
		sessionManager: sessionManager,
	}
}

type Service struct {
	sessionManager *user.SessionManager
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.Handle("GET /contrib/", http.StripPrefix("/contrib", http.FileServerFS(assets.FS)))
	mux.HandleFunc("GET /contrib/context", s.getContext)
}

func (s Service) getContext(response http.ResponseWriter, request *http.Request) {
	session := s.sessionManager.Get(request)
	if session == nil {
		http.Error(response, "no session found", http.StatusUnauthorized)
		return
	}
	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
	var patient fhir.Patient
	if err := fhirClient.Get(session.Values["patient"], &patient); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(patient)
	_, _ = response.Write([]byte(data))
}
