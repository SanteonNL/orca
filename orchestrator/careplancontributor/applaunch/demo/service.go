package demo

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strconv"
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

func New(sessionManager *user.SessionManager[session.Data], config Config, frontendLandingUrl *url.URL, profile profile.Provider) *Service {
	return &Service{
		sessionManager:     sessionManager,
		config:             config,
		frontendLandingUrl: frontendLandingUrl,
		profile:            profile,
		ehrFHIRClientFactory: func(baseURL *url.URL, httpClient *http.Client) fhirclient.Client {
			return fhirclient.New(baseURL, httpClient, nil)
		},
	}
}

type Service struct {
	sessionManager       *user.SessionManager[session.Data]
	config               Config
	baseURL              string
	frontendLandingUrl   *url.URL
	ehrFHIRClientFactory func(*url.URL, *http.Client) fhirclient.Client
	profile              profile.Provider
}

func (s *Service) cpsFhirClient() fhirclient.Client {
	return globals.CarePlanServiceFhirClient
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/demo-app-launch", s.handle)
}

func (s *Service) handle(response http.ResponseWriter, request *http.Request) {
	log.Ctx(request.Context()).Debug().Msg("Handling demo app launch")
	values, ok := getQueryParams(response, request, "patient", "serviceRequest", "practitioner", "iss")
	if !ok {
		return
	}
	sessionData := session.Data{
		FHIRLauncher: fhirLauncherKey,
		LauncherProperties: map[string]string{
			"iss": values["iss"],
		},
	}
	sessionData.Set(values["serviceRequest"], nil)
	// taskIdentifier is optional, only set if present
	if taskIdentifiers := request.URL.Query()["taskIdentifier"]; len(taskIdentifiers) > 0 {
		sessionData.TaskIdentifier = &taskIdentifiers[0]
	}

	//Destroy the previous session if found
	existingSession := s.sessionManager.Get(request)
	if existingSession != nil {
		log.Ctx(request.Context()).Debug().Msg("Demo launch performed and previous session found - Destroying previous session")
		s.sessionManager.Destroy(response, request)
	}

	ehrFHIRClientProps := clients.Factories[fhirLauncherKey](values)
	ehrFHIRClient := s.ehrFHIRClientFactory(ehrFHIRClientProps.BaseURL, &http.Client{Transport: ehrFHIRClientProps.Client})

	var practitioner fhir.Practitioner
	if err := ehrFHIRClient.Read(values["practitioner"], &practitioner); err != nil {
		log.Ctx(request.Context()).Error().Err(err).Msg("Failed to read practitioner resource")
		http.Error(response, "Failed to read practitioner resource: "+err.Error(), http.StatusBadRequest)
		return
	}
	sessionData.Set("Practitioner/"+*practitioner.Id, practitioner)

	var patient fhir.Patient
	if err := ehrFHIRClient.Read(values["patient"], &patient); err != nil {
		log.Ctx(request.Context()).Error().Err(err).Msg("Failed to read patient resource")
		http.Error(response, "Failed to read patient resource: "+err.Error(), http.StatusBadRequest)
		return
	}
	sessionData.Set("Patient/"+*patient.Id, patient)

	organizations, err := s.profile.Identities(request.Context())
	if err != nil {
		log.Ctx(request.Context()).Error().Err(err).Msg("Failed to get active organization")
		http.Error(response, "Failed to get active organization: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(organizations) != 1 {
		log.Ctx(request.Context()).Error().Msgf("Expected 1 active organization, found %d", len(organizations))
		http.Error(response, "Expected 1 active organization, found "+strconv.Itoa(len(organizations)), http.StatusInternalServerError)
		return
	}
	sessionData.Set("Organization/magic-"+uuid.NewString(), organizations[0])
	s.sessionManager.Create(response, sessionData)

	var existingTask *fhir.Task
	if sessionData.TaskIdentifier != nil {
		taskIdentifier, err := coolfhir.TokenToIdentifier(*sessionData.TaskIdentifier)
		if err != nil {
			http.Error(response, "Failed to parse task identifier: "+err.Error(), http.StatusBadRequest)
			return
		}
		existingTask, err = coolfhir.GetTaskByIdentifier(request.Context(), s.cpsFhirClient(), *taskIdentifier)
		if err != nil {
			log.Ctx(request.Context()).Error().Err(err).Msg("Existing CPS Task check failed for task with identifier: " + coolfhir.ToString(taskIdentifier))
			http.Error(response, "Failed to check for existing CPS Task resource", http.StatusInternalServerError)
			return
		}
	}

	if existingTask != nil {
		log.Ctx(request.Context()).Debug().Msg("Existing CPS Task resource found for demo task with identifier: " + values["taskIdentifier"])
		http.Redirect(response, request, s.frontendLandingUrl.JoinPath("task", *existingTask.Id).String(), http.StatusFound)
		return
	}

	// Redirect to landing page
	log.Ctx(request.Context()).Debug().Msg("No existing CPS Task resource found for demo task with identifier: " + values["taskIdentifier"])
	http.Redirect(response, request, s.frontendLandingUrl.JoinPath("new").String(), http.StatusFound)
}
