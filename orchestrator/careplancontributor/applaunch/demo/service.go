package demo

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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

func New(sessionManager *user.SessionManager[session.Data], config Config, tenants tenants.Config, orcaPublicURL *url.URL, frontendLandingUrl *url.URL, profile profile.Provider) *Service {
	return &Service{
		sessionManager:     sessionManager,
		config:             config,
		orcaPublicURL:      orcaPublicURL,
		frontendLandingUrl: frontendLandingUrl,
		profile:            profile,
		tenants:            tenants,
		ehrFHIRClientFactory: func(config coolfhir.ClientConfig) (fhirclient.Client, error) {
			_, client, err := coolfhir.NewAuthRoundTripper(config, coolfhir.Config())
			return client, err
		},
	}
}

type Service struct {
	sessionManager       *user.SessionManager[session.Data]
	config               Config
	tenants              tenants.Config
	orcaPublicURL        *url.URL
	frontendLandingUrl   *url.URL
	ehrFHIRClientFactory func(config coolfhir.ClientConfig) (fhirclient.Client, error)
	profile              profile.Provider
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/demo-app-launch", s.handle)
}

func (s *Service) handle(response http.ResponseWriter, request *http.Request) {
	slog.DebugContext(request.Context(), "Handling demo app launch")
	values, ok := getQueryParams(response, request, "patient", "practitioner", "tenant")
	if !ok {
		return
	}
	serviceRequest := request.URL.Query().Get("serviceRequest")
	if serviceRequest != "" {
		values["serviceRequest"] = serviceRequest
	}
	tenant, err := s.tenants.Get(values["tenant"])
	if err != nil {
		http.Error(response, "App launch failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	if tenant.Demo.FHIR.BaseURL == "" {
		http.Error(response, "App launch failed: FHIR base URL is not configured for tenant "+tenant.ID, http.StatusBadRequest)
		return
	}
	sessionData := session.Data{
		FHIRLauncher: fhirLauncherKey,
		TenantID:     tenant.ID,
		LauncherProperties: map[string]string{
			"iss": tenant.Demo.FHIR.BaseURL,
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
		slog.DebugContext(request.Context(), "Demo launch performed and previous session found - Destroying previous session")
		s.sessionManager.Destroy(response, request)
	}

	// Create FHIR client using the factory
	ehrFHIRClient, err := s.ehrFHIRClientFactory(tenant.Demo.FHIR)
	if err != nil {
		slog.ErrorContext(request.Context(), "Failed to create FHIR client", slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to create FHIR client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var practitioner fhir.Practitioner
	if err := ehrFHIRClient.Read(values["practitioner"], &practitioner); err != nil {
		slog.ErrorContext(request.Context(), "Failed to read practitioner resource", slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to read practitioner resource: "+err.Error(), http.StatusBadRequest)
		return
	}
	sessionData.Set("Practitioner/"+*practitioner.Id, practitioner)

	var patient fhir.Patient
	if err := ehrFHIRClient.Read(values["patient"], &patient); err != nil {
		slog.ErrorContext(request.Context(), "Failed to read patient resource", slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to read patient resource: "+err.Error(), http.StatusBadRequest)
		return
	}
	sessionData.Set("Patient/"+*patient.Id, patient)

	ctx := tenants.WithTenant(request.Context(), *tenant)

	organizations, err := s.profile.Identities(ctx)
	if err != nil {
		slog.ErrorContext(request.Context(), "Failed to get active organization", slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to get active organization: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(organizations) != 1 {
		slog.ErrorContext(request.Context(), fmt.Sprintf("Expected 1 active organization, found %d", len(organizations)))
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
		fhirClient, err := globals.CreateCPSFHIRClient(ctx)
		if err != nil {
			http.Error(response, "Failed to create FHIR client for existing Task check: "+err.Error(), http.StatusInternalServerError)
			return
		}
		existingTask, err = coolfhir.GetTaskByIdentifier(request.Context(), fhirClient, *taskIdentifier)
		if err != nil {
			slog.ErrorContext(
				request.Context(),
				"Existing CPS Task check failed for task",
				slog.String(logging.FieldError, err.Error()),
				slog.String(logging.FieldIdentifier, coolfhir.ToString(taskIdentifier)),
			)
			http.Error(response, "Failed to check for existing CPS Task resource", http.StatusInternalServerError)
			return
		}
	}

	if existingTask != nil {
		slog.DebugContext(request.Context(), "Existing CPS Task resource found for demo task", slog.String(logging.FieldIdentifier, values["taskIdentifier"]))
		http.Redirect(response, request, s.frontendLandingUrl.JoinPath("task", *existingTask.Id).String(), http.StatusFound)
		return
	}

	// Redirect to landing page
	if serviceRequest == "" {
		// No ServiceRequest given from EHR, redirect to task overview
		slog.DebugContext(request.Context(), "No ServiceRequest provided by EHR, redirecting to Task overview")
		http.Redirect(response, request, s.frontendLandingUrl.JoinPath("list").String(), http.StatusFound)
	} else {
		// ServiceRequest given, redirect to new enrollment landing page
		slog.DebugContext(request.Context(), "No existing CPS Task resource found for demo task", slog.String(logging.FieldIdentifier, values["taskIdentifier"]))
		http.Redirect(response, request, s.frontendLandingUrl.JoinPath("new").String(), http.StatusFound)
	}
}

func (s *Service) CreateEHRProxies() (map[string]coolfhir.HttpProxy, map[string]fhirclient.Client) {
	proxies := make(map[string]coolfhir.HttpProxy)
	fhirClients := make(map[string]fhirclient.Client)

	for _, tenant := range s.tenants {
		if tenant.Demo.FHIR.BaseURL == "" {
			continue
		}
		fhirBaseURL := must.ParseURL(tenant.Demo.FHIR.BaseURL)
		transport, fhirClient, err := coolfhir.NewAuthRoundTripper(tenant.Demo.FHIR, coolfhir.Config())
		if err != nil {
			slog.Error(
				"Failed to create FHIR client for tenant",
				slog.String("tenant", tenant.ID),
				slog.String("baseURL", fhirBaseURL.String()),
				slog.String(logging.FieldError, err.Error()),
			)
			continue
		}
		tenantBasePath := "/cpc/" + tenant.ID + "/fhir"
		proxy := coolfhir.NewProxy("App->EHR", fhirBaseURL, tenantBasePath, s.orcaPublicURL.JoinPath(tenantBasePath), transport, false, false)
		proxies[tenant.ID] = proxy
		fhirClients[tenant.ID] = fhirClient
	}

	return proxies, fhirClients
}
