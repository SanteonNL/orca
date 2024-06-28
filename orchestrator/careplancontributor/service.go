package careplancontributor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/assets"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

const LandingURL = "/contrib/"

type Service struct {
	SessionManager  *user.SessionManager
	CarePlanService fhirclient.Client
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

func (s Service) confirm(localFHIR fhirclient.Client, serviceRequestRef string, patientRef string) (*fhir.Task, error) {
	serviceRequest, err := s.readServiceRequest(localFHIR, serviceRequestRef)
	if err != nil {
		return nil, err
	}

	var patient fhir.Patient
	if err := localFHIR.Read(patientRef, &patient); err != nil {
		return nil, err
	}

	// TODO: Should we do this in a Bundle?
	//carePlan, err := s.createCarePlan(patient)
	//if err != nil {
	//	return nil, fmt.Errorf("failed to create CarePlan: %w", err)
	//}
	task, err := s.createTask(*serviceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create Task: %w", err)
	}

	// Start polling in a new goroutine so that the code continues to the select below
	err = s.pollTaskStatus(*task.Id)

	if err != nil {
		fmt.Printf("error while polling task %s: %v\n", *task.Id, err)
		return nil, err
	}

	enrichedTask, err := s.handleAcceptedTask(task, serviceRequest, &patient)
	return enrichedTask, err
}

// pollTaskStatus polls the task status until it is accepted, an error occurs or reaches a max poll amount.
func (s Service) pollTaskStatus(taskID string) error {
	pollInterval := 2 * time.Second
	maxPollingDuration := 20 * time.Second
	startTime := time.Now()

	for {
		if time.Since(startTime) >= maxPollingDuration {
			return fmt.Errorf("maximum polling duration of %d seconds reached for Task/%s", maxPollingDuration, taskID)
		}

		var task fhir.Task

		//TODO: Add timeout to the read when the lib supports it
		if err := s.CarePlanService.Read("Task/"+taskID, &task); err != nil {
			log.Error().Msgf("polling Task/%s failed - error: [%s]", taskID, err)
			return err
		}

		log.Info().Msgf("polling Task/%s - got status [%s]", taskID, &task.Status)

		if task.Status == fhir.TaskStatusAccepted {
			return nil
		}

		time.Sleep(pollInterval)
	}
}

func (s Service) readServiceRequest(localFHIR fhirclient.Client, serviceRequestRef string) (*fhir.ServiceRequest, error) {
	// TODO: Make this complete, and test this
	var serviceRequest fhir.ServiceRequest
	if err := localFHIR.Read(serviceRequestRef, &serviceRequest); err != nil {
		return nil, err
	}

	serviceRequest.ReasonReference = nil
	var serviceRequestReasons []map[string]interface{}
	for i, reasonReference := range serviceRequest.ReasonReference {
		// TODO: ReasonReference should probably be an ID instead of logical identifier?
		if reasonReference.Identifier == nil || reasonReference.Identifier.Value == nil {
			return nil, fmt.Errorf("expected ServiceRequest.reasonReference[%d].identifier.value to be set", i)
		}
		results := make([]map[string]interface{}, 0)
		// TODO: Just taking first isn't right, fix with technical IDs instead of logical identifiers
		if err := localFHIR.Read(*reasonReference.Type+"/?identifier="+*reasonReference.Identifier.Value, &results); err != nil {
			return nil, err
		}
		if len(results) == 0 {
			return nil, fmt.Errorf("could not resolve ServiceRequest.reasonReference[%d].identifier", i)
		}
		reason := results[0]
		ref := fmt.Sprintf("#servicerequest-reason-%d", i+1)
		reason["id"] = ref
		serviceRequestReasons = append(serviceRequestReasons, results[0])
		serviceRequest.ReasonReference = append(serviceRequest.ReasonReference, fhir.Reference{
			Type:      to.Ptr(*reasonReference.Type),
			Reference: to.Ptr(ref),
		})
	}
	return &serviceRequest, nil
}

func (s Service) createCarePlan(patient fhir.Patient) (*fhir.CarePlan, error) {
	carePlan := fhir.CarePlan{
		Subject: fhir.Reference{
			Type:      to.Ptr("Patient"),
			Reference: patient.Id,
		},
	}
	var result fhir.CarePlan
	if err := s.CarePlanService.Create(carePlan, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s Service) createTask(serviceRequest fhir.ServiceRequest) (*fhir.Task, error) {
	// Marshalling of Task is incorrect when providing input
	// See https://github.com/samply/golang-fhir-models/issues/19
	// So just create a regular map.

	// TODO: Should we make new cross references here for requester, owner, service request and patient?

	task := map[string]any{
		"resourceType": "Task",
		"status":       "requested",
		"intent":       "order",
		"requester":    serviceRequest.Requester,
		"owner":        serviceRequest.Performer,
		"reasonCode":   serviceRequest.Code,
	}
	taskJSON, _ := json.MarshalIndent(task, "", "  ")
	println(string(taskJSON))
	createdTask, err := coolfhir.Workflow{CarePlanService: s.CarePlanService}.Invoke(task)
	return createdTask, err
}

// When an application accepts the Task, we enrich the Task with more detailed information like the actual Patient and the ServiceRequest
func (s Service) handleAcceptedTask(task *fhir.Task, serviceRequest *fhir.ServiceRequest, patient *fhir.Patient) (*fhir.Task, error) {

	taskMap, err := s.enrichTask(task, serviceRequest, patient)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich task: %w", err)
	}

	var enrichedTask fhir.Task
	if err := s.CarePlanService.Update("Task/"+*task.Id, *taskMap, &enrichedTask); err != nil {
		return nil, fmt.Errorf("failed to update Task: %w", err)
	}

	// s.updateCarePlanWithTask(result)

	//carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
	//	Reference: &fhir.Reference{
	//		Type:      to.Ptr("Task"),
	//		Reference: to.Ptr("Task/" + *task.Id), // TODO: This seems a fiddly way to reference stuff
	//	},
	//})
	//// TODO: Add "If" headers to make sure we're not overwriting someone else's changes
	//if err := s.CarePlanService.Update("CarePlan/"+*carePlan.Id, carePlan, &carePlan); err != nil {
	//	return nil, fmt.Errorf("failed to add Task to CarePlan: %w", err)
	//}

	return &enrichedTask, nil
}

func (s Service) enrichTask(task *fhir.Task, serviceRequest *fhir.ServiceRequest, patient *fhir.Patient) (*map[string]interface{}, error) {

	//FIXME: fhir.Task.Contained does not exist for some reason - converting the task to a map for easy manipulation instead
	taskMap := make(map[string]interface{})
	taskJSON, _ := json.Marshal(task)
	if err := json.Unmarshal(taskJSON, &taskMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	taskMap["for"] = fhir.Reference{
		Type:      to.Ptr("Patient"),
		Reference: to.Ptr("#" + *patient.Id), //convert to local reference
	}

	taskMap["input"] = []map[string]interface{}{
		{
			"valueReference": fhir.Reference{
				Type:      to.Ptr("ServiceRequest"),
				Reference: to.Ptr("#" + *serviceRequest.Id), //convert to local reference
			},
		},
	}

	taskMap["contained"] = []interface{}{
		*serviceRequest,
		*patient,
	}

	return &taskMap, nil
}

func unmarshalFromBundle(bundle fhir.Bundle, resourceType string, target any) error {
	type Base struct {
		ResourceType string `json:"resourceType"`
	}
	for _, entry := range bundle.Entry {
		entryJSON, _ := entry.Resource.MarshalJSON()
		var base Base
		if err := json.Unmarshal(entryJSON, &base); err != nil {
			return fmt.Errorf("unable to unmarshal base resource: %w", err)
		}
		if base.ResourceType == resourceType {
			if err := json.Unmarshal(entryJSON, target); err != nil {
				return fmt.Errorf("unable to unmarshal bundle entry (resourceType=%s) into %T: %w", resourceType, target, err)
			}
			return nil
		}
	}
	return fmt.Errorf("entry not found in bundle (resourceType=%s)", resourceType)
}
