package demo

import (
	"fmt"
	"net/http"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const fhirLauncherKey = "demo"
const demoTaskSystemSystem = "http://demo-launch/fhir/NamingSystem/task-identifier"

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

func New(sessionManager *user.SessionManager, config Config, frontendLandingUrl string) *Service {
	return &Service{
		sessionManager:     sessionManager,
		config:             config,
		frontendLandingUrl: frontendLandingUrl,
	}
}

type Service struct {
	sessionManager     *user.SessionManager
	config             Config
	baseURL            string
	frontendLandingUrl string
}

func (s *Service) cpsFhirClient() fhirclient.Client {
	return globals.CarePlanServiceFhirClient
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/demo-app-launch", s.handle)
}

func (s *Service) handle(response http.ResponseWriter, request *http.Request) {
	log.Debug().Ctx(request.Context()).Msg("Handling demo app launch")
	values, ok := getQueryParams(response, request, "patient", "serviceRequest", "practitioner", "iss", "taskIdentifier")
	if !ok {
		return
	}

	//Destroy the previous session if found
	session := s.sessionManager.Get(request)
	if session != nil {
		log.Debug().Ctx(request.Context()).Msg("Demo launch performed and previous session found - Destroying previous session")
		s.sessionManager.Destroy(response, request)
	}

	s.sessionManager.Create(response, user.SessionData{
		FHIRLauncher: fhirLauncherKey,
		StringValues: values,
	})

	var existingTaskBundle fhir.Bundle
	headers := http.Header{}
	headers.Add("Cache-Control", "no-cache")
	err := s.cpsFhirClient().SearchWithContext(request.Context(), "Task", url.Values{
		"identifier": []string{demoTaskSystemSystem + "|" + values["taskIdentifier"]},
	}, &existingTaskBundle, fhirclient.RequestHeaders(headers))

	log.Debug().Ctx(request.Context()).Msg("Search for existing CPS Task resource for demo task completed")
	if err != nil {
		http.Error(response, fmt.Sprintf("Failed to search for Task: %v", err), http.StatusInternalServerError)
		return
	}

	if len(existingTaskBundle.Entry) == 1 {
		log.Debug().Ctx(request.Context()).Msg("Found existing CPS Task resource for demo task")
		var existingTask fhir.Task
		if err := coolfhir.ResourceInBundle(&existingTaskBundle, coolfhir.EntryIsOfType("Task"), &existingTask); err != nil {
			http.Error(response, fmt.Sprintf("unable to get existing CPS Task resource from search bundle: %v", err), http.StatusInternalServerError)
			return
		}
		http.Redirect(response, request, s.frontendLandingUrl+"/task/"+*existingTask.Id, http.StatusFound)
		return
	} else if len(existingTaskBundle.Entry) > 1 {
		log.Error().Ctx(request.Context()).Msg("Found multiple existing CPS Tasks for demo task with identifier: " + values["taskIdentifier"])
		http.Error(response, fmt.Sprintf("found multiple existing CPS Tasks for demo task (taskIdentifier=%s): %v", values["taskIdentifier"], err), http.StatusConflict)
		return
	}

	// Redirect to landing page
	log.Debug().Ctx(request.Context()).Msg("No existing CPS Task resource found for demo task with identifier: " + values["taskIdentifier"])
	http.Redirect(response, request, s.frontendLandingUrl+"/new", http.StatusFound)
}
