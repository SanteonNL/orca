//go:generate mockgen -destination=./mock/fhirclient_mock.go -package=mock github.com/SanteonNL/go-fhir-client Client
package careplancontributor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/nuts-foundation/go-nuts-client/oauth2"
	"net/http"
	"net/url"
	"time"

	"github.com/SanteonNL/orca/orchestrator/addressing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/assets"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

const LandingURL = "/contrib/"
const CarePlanServiceOAuth2Scope = "careplanservice"

func New(config Config, sessionManager *user.SessionManager, carePlanServiceHttpClient *http.Client, didResolver addressing.StaticDIDResolver) *Service {
	cpsURL, _ := url.Parse(config.CarePlanService.URL)
	carePlanServiceClient := fhirclient.New(cpsURL, carePlanServiceHttpClient, coolfhir.Config())
	return &Service{
		carePlanServiceURL:        cpsURL,
		SessionManager:            sessionManager,
		carePlanService:           carePlanServiceClient,
		carePlanServiceHttpClient: carePlanServiceHttpClient,
		frontendUrl:               config.FrontendConfig.URL,
	}
}

type Service struct {
	SessionManager            *user.SessionManager
	frontendUrl               string
	carePlanService           fhirclient.Client
	carePlanServiceURL        *url.URL
	carePlanServiceHttpClient *http.Client
}

func (s Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /contrib/context", s.withSession(s.handleGetContext))
	mux.HandleFunc("GET /contrib/patient", s.withSession(s.handleGetPatient))
	mux.HandleFunc("GET /contrib/practitioner", s.withSession(s.handleGetPractitioner))
	mux.HandleFunc("GET /contrib/serviceRequest", s.withSession(s.handleGetServiceRequest))
	mux.HandleFunc("POST /contrib/confirm", s.withSession(s.handleConfirm))
	mux.HandleFunc("/contrib/ehr/fhir/*", s.withSession(s.handleProxyToEPD))
	carePlanServiceProxy := coolfhir.NewProxy(log.Logger, s.carePlanServiceURL, "/contrib/cps/fhir", s.carePlanServiceHttpClient.Transport)
	mux.HandleFunc("/contrib/cps/fhir/*", s.withSession(func(writer http.ResponseWriter, request *http.Request, _ *user.SessionData) {
		carePlanServiceProxy.ServeHTTP(writer, request)
	}))
	mux.HandleFunc("/contrib/", func(response http.ResponseWriter, request *http.Request) {
		log.Info().Msgf("Redirecting to %s", s.frontendUrl)
		http.Redirect(response, request, s.frontendUrl, http.StatusFound)
	})
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

func (s Service) handleProxyToEPD(writer http.ResponseWriter, request *http.Request, session *user.SessionData) {
	clientFactory := clients.Factories[session.FHIRLauncher](session.Values)
	proxy := coolfhir.NewProxy(log.Logger, clientFactory.BaseURL, "/contrib/ehr/fhir", clientFactory.Client)
	proxy.ServeHTTP(writer, request)
}

// handleGetPatient is the REST API handler that returns the FHIR Patient.
func (s Service) handleGetPatient(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())

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

// handleGetPractitioner is the REST API handler that returns the FHIR Practitioner.
func (s Service) handleGetPractitioner(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())
	var practitioner fhir.Practitioner
	if err := fhirClient.Read(session.Values["practitioner"], &practitioner); err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(practitioner)
	_, _ = response.Write(data)
}

// handleGetServiceRequest is the REST API handler that returns the FHIR ServiceRequest.
func (s Service) handleGetServiceRequest(response http.ResponseWriter, request *http.Request, session *user.SessionData) {
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())
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
	// TODO: Remove this when frontend works on the proxy endpoint
	clientProperties := clients.Factories[session.FHIRLauncher](session.Values)
	fhirClient := fhirclient.New(clientProperties.BaseURL, &http.Client{Transport: clientProperties.Client}, coolfhir.Config())
	err := s.confirm(fhirClient, session.Values["serviceRequest"], session.Values["patient"])
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

func (s Service) confirm(localFHIR fhirclient.Client, serviceRequestRef string, patientRef string) error {
	serviceRequest, err := s.readServiceRequest(localFHIR, serviceRequestRef)
	if err != nil {
		return fmt.Errorf("could not resolve ServiceRequest: %w", err)
	}

	var patient fhir.Patient
	if err := localFHIR.Read(patientRef, &patient); err != nil {
		return fmt.Errorf("could not resolve Patient: %w", err)
	}

	// TODO: Should we do this in a Bundle?
	carePlan, err := s.createCarePlan(patient)
	if err != nil {
		return fmt.Errorf("failed to create CarePlan: %w", err)
	}
	task, err := s.createTask(*serviceRequest, *carePlan.Id)
	if err != nil {
		return fmt.Errorf("failed to create Task at CarePlanService: %w", err)
	}

	// Start polling in a new goroutine so that the code continues to the select below
	err = s.pollTaskStatus(*task.Id)

	if err != nil {
		return fmt.Errorf("error while polling task %s: %w", *task.Id, err)
	}

	_, err = s.handleAcceptedTask(task, serviceRequest, &patient)
	return err
}

// pollTaskStatus polls the task status until it is accepted, an error occurs or reaches a max poll amount.
func (s Service) pollTaskStatus(taskID string) error {
	pollInterval := 2 * time.Second
	maxPollingDuration := 20 * time.Second
	startTime := time.Now()
	ctx := oauth2.WithScope(context.Background(), CarePlanServiceOAuth2Scope)
	ctx = oauth2.WithResourceURI(ctx, s.carePlanServiceURL.String())
	for {
		if time.Since(startTime) >= maxPollingDuration {
			return fmt.Errorf("maximum polling duration of %s reached for Task/%s", maxPollingDuration, taskID)
		}

		var task fhir.Task

		//TODO: Add timeout to the read when the lib supports it
		if err := s.carePlanService.ReadWithContext(ctx, "Task/"+taskID, &task); err != nil {
			return fmt.Errorf("polling Task/%s failed - error: [%w]", taskID, err)
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

func (s Service) handleGetContext(response http.ResponseWriter, _ *http.Request, session *user.SessionData) {
	contextData := struct {
		Patient        string `json:"patient"`
		ServiceRequest string `json:"serviceRequest"`
		Practitioner   string `json:"practitioner"`
	}{
		Patient:        session.Values["patient"],
		ServiceRequest: session.Values["serviceRequest"],
		Practitioner:   session.Values["practitioner"],
	}
	response.Header().Add("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(contextData)
}

func (s Service) createCarePlan(patient fhir.Patient) (*fhir.CarePlan, error) {
	patientBSN := coolfhir.FirstIdentifier(patient.Identifier, coolfhir.IsNamingSystem(coolfhir.BSNNamingSystem))
	if patientBSN == nil {
		return nil, errors.New("patient is missing identifier of type " + coolfhir.BSNNamingSystem)
	}

	carePlan := fhir.CarePlan{
		Subject: fhir.Reference{
			Type:       to.Ptr("Patient"),
			Identifier: patientBSN,
		},
	}
	var result fhir.CarePlan
	ctx := oauth2.WithScope(context.Background(), CarePlanServiceOAuth2Scope)
	ctx = oauth2.WithResourceURI(ctx, s.carePlanServiceURL.String())
	if err := s.carePlanService.CreateWithContext(ctx, carePlan, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s Service) createTask(serviceRequest fhir.ServiceRequest, carePlanID string) (*fhir.Task, error) {
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
		"basedOn": []fhir.Reference{
			{
				Type:      to.Ptr("CarePlan"),
				Reference: to.Ptr("CarePlan/" + carePlanID),
			},
		},
	}
	ctx := oauth2.WithScope(context.Background(), CarePlanServiceOAuth2Scope)
	ctx = oauth2.WithResourceURI(ctx, s.carePlanServiceURL.String())
	createdTask, err := coolfhir.Workflow{CarePlanService: s.carePlanService}.Invoke(ctx, task)
	return createdTask, err
}

// When an application accepts the Task, we enrich the Task with more detailed information like the actual Patient and the ServiceRequest
func (s Service) handleAcceptedTask(task *fhir.Task, serviceRequest *fhir.ServiceRequest, patient *fhir.Patient) (*fhir.Task, error) {
	taskMap, err := s.enrichTask(task, serviceRequest, patient)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich task: %w", err)
	}
	var enrichedTask fhir.Task
	ctx := oauth2.WithScope(context.Background(), CarePlanServiceOAuth2Scope)
	ctx = oauth2.WithResourceURI(ctx, s.carePlanServiceURL.String())
	if err := s.carePlanService.UpdateWithContext(ctx, "Task/"+*task.Id, *taskMap, &enrichedTask); err != nil {
		return nil, fmt.Errorf("failed to update Task: %w", err)
	}
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
