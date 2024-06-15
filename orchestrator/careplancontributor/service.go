package careplancontributor

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/assets"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
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
	mux.HandleFunc("GET /contrib/patient", s.getPatient)
	mux.HandleFunc("GET /contrib/serviceRequest", s.getServiceRequest)
	mux.HandleFunc("POST /contrib/confirm", s.confirm)
	//mux.HandleFunc("GET /contrib/fhir", s.fhirProxy)
}

func (s Service) getPatient(response http.ResponseWriter, request *http.Request) {
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
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(patient)
	_, _ = response.Write(data)
}

func (s Service) getServiceRequest(response http.ResponseWriter, request *http.Request) {
	session := s.sessionManager.Get(request)
	if session == nil {
		http.Error(response, "no session found", http.StatusUnauthorized)
		return
	}
	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
	var serviceRequest fhir.ServiceRequest
	if err := fhirClient.Get(session.Values["serviceRequest"], &serviceRequest); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(serviceRequest)
	_, _ = response.Write(data)
}

func (s Service) confirm(response http.ResponseWriter, request *http.Request) {
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte("TODO"))
}

// Alternatively:
//func (s Service) fhirProxy(response http.ResponseWriter, request *http.Request) {
//	session := s.sessionManager.Get(request)
//	if session == nil {
//		http.Error(response, "no session found", http.StatusUnauthorized)
//		return
//	}
//	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
//
//	// Proxy the request to the FHIR server
//	(&httputil.ReverseProxy{
//		Rewrite: func(r *httputil.ProxyRequest) {
//			r.SetURL(fhirClient.BaseURL)
//		},
//		Transport: fhirClient.HTTPClient.Transport,
//	}).ServeHTTP(response, request)
//}
