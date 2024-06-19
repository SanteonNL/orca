package careplancontributor

import (
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/assets"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

const LandingURL = "/contrib/"

type Service struct {
	SessionManager  *user.SessionManager
	CarePlanService coolfhir.Client
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.Handle("GET /contrib/", http.StripPrefix("/contrib", http.FileServerFS(assets.FS)))
	mux.HandleFunc("GET /contrib/patient", s.withSession(s.handleGetPatient))
	mux.HandleFunc("GET /contrib/serviceRequest", s.withSession(s.handleGetServiceRequest))
	mux.HandleFunc("POST /contrib/confirm", s.withSession(s.handleConfirm))
}

// withSession is a middleware that retrieves the session for the given request.
// It then calls the given handler function and provides the session.
// If there's no active session, it returns a 401 Unauthorized response.
func (s Service) withSession(next func(response http.ResponseWriter, request *http.Request, session *user.SessionData)) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		session := s.SessionManager.Get(request)
		if session == nil {
			http.Error(response, "no session found", http.StatusUnauthorized)
			return
		}
		next(response, request, session)
	}
}

// handleGetPatient is the REST API handler that returns the FHIR Patient.
func (s Service) handleGetPatient(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
	var patient fhir.Patient
	if err := fhirClient.Read(session.Values["patient"], &patient); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(patient)
	_, _ = response.Write(data)
}

// handleGetServiceRequest is the REST API handler that returns the FHIR ServiceRequest.
func (s Service) handleGetServiceRequest(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
	var serviceRequest fhir.ServiceRequest
	if err := fhirClient.Read(session.Values["serviceRequest"], &serviceRequest); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(serviceRequest)
	_, _ = response.Write(data)
}

// handleConfirm is the REST API handler that handles workflow invocation confirmation,
// and initiates the workflow.
func (s Service) handleConfirm(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	fhirClient := coolfhir.ClientFactories[session.FHIRLauncher](session.Values)
	_, err := s.confirm(fhirClient, session.Values["serviceRequest"], session.Values["patient"])
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	data, err := assets.FS.ReadFile("completed.html")
	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(data)
}

func (s Service) confirm(localFHIR coolfhir.Client, serviceRequestRef string, patientRef string) (*fhir.Task, error) {
	// TODO: Make this complete, and test this
	var serviceRequest fhir.ServiceRequest
	if err := localFHIR.Read(serviceRequestRef, &serviceRequest); err != nil {
		return nil, err
	}
	var patient fhir.Patient
	if err := localFHIR.Read(patientRef, &patient); err != nil {
		return nil, err
	}
	// TODO: Should we do this in a Bundle?
	carePlan, err := s.createCarePlan(patient)
	if err != nil {
		return nil, fmt.Errorf("failed to create CarePlan: %w", err)
	}
	task, err := s.createTask(serviceRequestRef)
	if err != nil {
		return nil, fmt.Errorf("failed to create Task: %w", err)
	}
	// Add Task to CarePlan
	// Note: since we created the Task with status=accepted, we can do this immediately.
	// Otherwise, we have to poll/wait from the UI.
	carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
		Reference: &fhir.Reference{
			Type:      to.Ptr("Task"),
			Reference: to.Ptr("Task/" + *task.Id), // TODO: This seems a fiddly way to reference stuff
		},
	})
	// TODO: Add "If" headers to make sure we're not overwriting someone else's changes
	if err := s.CarePlanService.Update("CarePlan/"+*carePlan.Id, carePlan, &carePlan); err != nil {
		return nil, fmt.Errorf("failed to add Task to CarePlan: %w", err)
	}
	return task, err
}

func (s Service) createCarePlan(patient fhir.Patient) (*fhir.CarePlan, error) {
	carePlan := fhir.CarePlan{
		Subject: fhir.Reference{
			Type:       to.Ptr("Patient"),
			Identifier: &patient.Identifier[0], // TODO: is this the right way/one?
		},
	}
	var result fhir.CarePlan
	if err := s.CarePlanService.Create("CarePlan", carePlan, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s Service) createTask(serviceRequestRef string) (*fhir.Task, error) {
	// Marshalling of Task is incorrect when providing input
	// See https://github.com/samply/golang-fhir-models/issues/19
	// So just create a regular map.
	task := map[string]any{
		"resourceType": "Task",
		"status":       "accepted",
		"intent":       "order",
		"requester":    coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"), // TODO: Take URA from config/request
		"owner":        coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"), // TODO: Take URA from config/request
		"input": []map[string]any{
			{
				"valueReference": fhir.Reference{
					Type:      to.Ptr("ServiceRequest"),
					Reference: to.Ptr(serviceRequestRef),
				},
			},
		},
	}
	createdTask, err := coolfhir.Workflow{CarePlanService: s.CarePlanService}.Invoke(task)
	return createdTask, err
}
