package verification

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/oidc/rp"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

const fhirLauncherKey = "verification"
const launchURLTTL = time.Minute

const contentTypeJSON = "application/json"

// fhirBaseURLIdentifierSystem addresses an SCP node by its FHIR base URL, which the profile resolves
// to an authorization server through the node's CapabilityStatement.
const fhirBaseURLIdentifierSystem = "https://build.fhir.org/http.html#root"

func init() {
	// Fallback only: every resource the frontend needs is cached in the session, so this client should
	// never be exercised. It points at the local CPS so a miss fails loudly against real data.
	clients.Factories[fhirLauncherKey] = func(properties map[string]string) clients.ClientProperties {
		baseURL, _ := url.Parse(properties["cps"])
		return clients.ClientProperties{
			BaseURL: baseURL,
			Client:  http.DefaultTransport,
		}
	}
}

// tokenValidator is the seam for OIDC token validation, satisfied by rp.Client.
type tokenValidator interface {
	ValidateToken(ctx context.Context, tokenString string, options ...rp.ValidateTokenOption) (*rp.TokenClaims, error)
}

// Service implements an authenticated app launch used to verify the EHR flow without a hospital:
// a workforce user proves who they are with an OIDC token (Entra), the launch session is built from
// the local CPS's own Task/Patient — mirroring a real launch — and a single-use URL bridges the
// authenticated call into a browser session. Unlike the demo launch this can be enabled in production.
type Service struct {
	sessionManager     *user.SessionManager[session.Data]
	config             Config
	tenants            tenants.Config
	orcaPublicURL      *url.URL
	frontendLandingUrl *url.URL
	profile            profile.Provider
	tokenValidator     tokenValidator
	nonces             *nonceStore
}

func New(
	ctx context.Context,
	config Config,
	sessionManager *user.SessionManager[session.Data],
	tenantsConfig tenants.Config,
	orcaPublicURL *url.URL,
	frontendLandingUrl *url.URL,
	profile profile.Provider,
) (*Service, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var oidcClient tokenValidator
	if config.OIDC.Enabled {
		client, err := rp.NewClient(ctx, &config.OIDC)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC client for verification app launch: %w", err)
		}
		oidcClient = client
	}
	return &Service{
		sessionManager:     sessionManager,
		config:             config,
		tenants:            tenantsConfig,
		orcaPublicURL:      orcaPublicURL,
		frontendLandingUrl: frontendLandingUrl,
		profile:            profile,
		tokenValidator:     oidcClient,
		nonces:             newNonceStore(launchURLTTL),
	}, nil
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	// Entrypoint for our own workforce application, authenticated with an OIDC token. Only registered
	// where that application can reach us; the launch is delegated when another node owns the Task.
	if s.tokenValidator != nil {
		mux.HandleFunc("POST /verification-launch", s.handleCreate)
	}
	// Entrypoint for another SCP node delegating a launch, authenticated over Shared Care Planning's
	// existing trust: no identity provider of the calling party has to be trusted here.
	mux.HandleFunc("POST /verification-launch/scp", s.profile.Authenticator(s.handleCreateForNode))
	mux.HandleFunc("GET /verification-launch/{nonce}", s.handleRedeem)
}

func (s *Service) CreateEHRProxies() (map[string]coolfhir.HttpProxy, map[string]fhirclient.Client) {
	// there is no EHR behind a verification launch; external SCP nodes have nothing to query here
	return map[string]coolfhir.HttpProxy{}, map[string]fhirclient.Client{}
}

type createLaunchRequest struct {
	Tenant string `json:"tenant"`
	TaskID string `json:"taskId"`
	// TaskReference is the absolute URL of the enrollment Task. When it belongs to another SCP node
	// the launch is delegated there, so the session, frontend and OIDC issuer stay with the care
	// organization that owns the enrollment — exactly as in a launch from its own EHR.
	TaskReference string `json:"taskReference"`
	// PractitionerName is asserted by the caller: app tokens (client credentials) carry no user
	// claims, so the calling system passes along who initiated the launch.
	PractitionerName string `json:"practitionerName"`
}

type createLaunchResponse struct {
	LaunchURL        string `json:"launchUrl"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

func (s *Service) handleCreate(response http.ResponseWriter, request *http.Request) {
	claims, ok := s.authenticate(response, request)
	if !ok {
		return
	}
	launchRequest, ok := decodeLaunchRequest(response, request)
	if !ok {
		return
	}
	launchRequest.PractitionerName = cmp.Or(launchRequest.PractitionerName, claims.Name, "Backoffice verification")
	s.createLaunch(response, request, launchRequest, claims.Subject, true)
}

// handleCreateForNode serves a launch delegated by another SCP node. It never delegates onwards, so a
// launch can't be bounced between nodes, and it trusts the calling node for the practitioner's name
// the same way the rest of Shared Care Planning trusts it for the data it sends.
func (s *Service) handleCreateForNode(response http.ResponseWriter, request *http.Request) {
	principal, err := auth.PrincipalFromContext(request.Context())
	if err != nil {
		http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	launchRequest, ok := decodeLaunchRequest(response, request)
	if !ok {
		return
	}
	launchRequest.PractitionerName = cmp.Or(launchRequest.PractitionerName, "Verification launch")
	s.createLaunch(response, request, launchRequest, to.EmptyString(principal.Organization.Name), false)
}

func decodeLaunchRequest(response http.ResponseWriter, request *http.Request) (createLaunchRequest, bool) {
	var launchRequest createLaunchRequest
	if err := json.NewDecoder(request.Body).Decode(&launchRequest); err != nil {
		http.Error(response, "Invalid request body", http.StatusBadRequest)
		return launchRequest, false
	}
	if launchRequest.TaskReference == "" && (launchRequest.Tenant == "" || launchRequest.TaskID == "") {
		http.Error(response, "taskReference, or tenant and taskId, are required", http.StatusBadRequest)
		return launchRequest, false
	}
	return launchRequest, true
}

func (s *Service) createLaunch(response http.ResponseWriter, request *http.Request, launchRequest createLaunchRequest, initiatedBy string, mayDelegate bool) {
	ctx := request.Context()

	tenant, taskID, remoteCPSURL, err := s.resolveTask(launchRequest)
	if err != nil {
		http.Error(response, err.Error(), http.StatusBadRequest)
		return
	}

	if remoteCPSURL != nil {
		if !mayDelegate {
			http.Error(response, "Task is not held by this node", http.StatusBadRequest)
			return
		}
		// Delegating acquires an access token as one of our own care organizations, so the tenant we
		// act as has to be in context.
		actingTenant, err := s.actingTenant(launchRequest.Tenant)
		if err != nil {
			http.Error(response, err.Error(), http.StatusBadRequest)
			return
		}
		s.delegateLaunch(tenants.WithTenant(ctx, *actingTenant), response, remoteCPSURL, launchRequest)
		return
	}

	ctx = tenants.WithTenant(ctx, *tenant)
	sessionData, err := s.buildSession(ctx, *tenant, taskID, launchRequest.PractitionerName)
	if err != nil {
		slog.ErrorContext(ctx, "Verification launch failed", slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to prepare launch", http.StatusUnprocessableEntity)
		return
	}

	nonce := s.nonces.Put(*sessionData)
	slog.InfoContext(ctx, "Verification launch prepared",
		slog.String("tenant", tenant.ID),
		slog.String("task_id", taskID),
		slog.String("initiated_by", initiatedBy),
		slog.String("initiated_by_name", launchRequest.PractitionerName))

	response.Header().Set("Content-Type", contentTypeJSON)
	_ = json.NewEncoder(response).Encode(createLaunchResponse{
		LaunchURL:        s.orcaPublicURL.JoinPath("verification-launch", nonce).String(),
		ExpiresInSeconds: int(launchURLTTL.Seconds()),
	})
}

// actingTenant picks the local care organization to delegate as: the requested one, or the only one
// this node serves.
func (s *Service) actingTenant(requested string) (*tenants.Properties, error) {
	if requested != "" {
		tenant, err := s.tenants.Get(requested)
		if err != nil {
			return nil, errors.New("Invalid tenant")
		}
		return tenant, nil
	}
	if len(s.tenants) != 1 {
		return nil, errors.New("tenant is required to delegate this launch")
	}
	for _, tenant := range s.tenants {
		return &tenant, nil
	}
	return nil, errors.New("tenant is required to delegate this launch")
}

// resolveTask determines which CarePlanService holds the Task. It returns either a local tenant, or
// the FHIR base URL of the remote CarePlanService that does.
func (s *Service) resolveTask(launchRequest createLaunchRequest) (*tenants.Properties, string, *url.URL, error) {
	if launchRequest.TaskReference == "" {
		tenant, err := s.tenants.Get(launchRequest.Tenant)
		if err != nil {
			return nil, "", nil, errors.New("Invalid tenant")
		}
		return tenant, launchRequest.TaskID, nil, nil
	}

	cpsBaseURL, taskID, err := splitTaskReference(launchRequest.TaskReference)
	if err != nil {
		return nil, "", nil, err
	}
	for _, tenant := range s.tenants {
		if tenant.URL(s.orcaPublicURL, careplanservice.FHIRBaseURL).String() == cpsBaseURL.String() {
			return &tenant, taskID, nil, nil
		}
	}
	return nil, taskID, cpsBaseURL, nil
}

func splitTaskReference(taskReference string) (*url.URL, string, error) {
	baseURL, reference, err := coolfhir.ParseExternalLiteralReference(taskReference, "Task")
	if err != nil {
		return nil, "", errors.New("taskReference must point at a Task")
	}
	if baseURL.Scheme != "https" || baseURL.Host == "" {
		return nil, "", errors.New("taskReference must be an absolute https URL")
	}
	return baseURL, strings.TrimPrefix(reference, "Task/"), nil
}

// delegateLaunch asks the node holding the Task to create the launch, and relays its response: the
// returned URL lives on that node, so redeeming it lands the user in its frontend and its OIDC flow.
func (s *Service) delegateLaunch(ctx context.Context, response http.ResponseWriter, cpsBaseURL *url.URL, launchRequest createLaunchRequest) {
	httpClient, err := s.profile.HttpClient(ctx, fhir.Identifier{
		System: to.Ptr(fhirBaseURLIdentifierSystem),
		Value:  to.Ptr(cpsBaseURL.String()),
	})
	if err != nil {
		slog.ErrorContext(ctx, "Verification launch delegation failed: no client for remote node",
			slog.String("cps", cpsBaseURL.String()), slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to reach the node holding this Task", http.StatusBadGateway)
		return
	}

	body, _ := json.Marshal(createLaunchRequest{
		TaskReference:    launchRequest.TaskReference,
		PractitionerName: launchRequest.PractitionerName,
	})
	targetURL := nodeBaseURL(cpsBaseURL).JoinPath("verification-launch", "scp").String()
	remoteResponse, err := httpClient.Post(targetURL, contentTypeJSON, bytes.NewReader(body))
	if err != nil {
		slog.ErrorContext(ctx, "Verification launch delegation failed",
			slog.String("target", targetURL), slog.String(logging.FieldError, err.Error()))
		http.Error(response, "Failed to reach the node holding this Task", http.StatusBadGateway)
		return
	}
	defer remoteResponse.Body.Close()

	if remoteResponse.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "Verification launch rejected by remote node",
			slog.String("target", targetURL), slog.Int("status", remoteResponse.StatusCode))
		http.Error(response, "The node holding this Task rejected the launch", http.StatusBadGateway)
		return
	}

	var launch createLaunchResponse
	if err := json.NewDecoder(remoteResponse.Body).Decode(&launch); err != nil || launch.LaunchURL == "" {
		http.Error(response, "The node holding this Task returned an invalid launch", http.StatusBadGateway)
		return
	}

	slog.InfoContext(ctx, "Verification launch delegated",
		slog.String("cps", cpsBaseURL.String()),
		slog.String("initiated_by_name", launchRequest.PractitionerName))

	response.Header().Set("Content-Type", contentTypeJSON)
	_ = json.NewEncoder(response).Encode(launch)
}

// nodeBaseURL strips the "/cps/{tenant}" suffix a CarePlanService FHIR base URL carries, leaving the
// node's public base URL.
func nodeBaseURL(cpsBaseURL *url.URL) *url.URL {
	base := *cpsBaseURL
	base.Path = path.Dir(path.Dir(strings.TrimSuffix(base.Path, "/")))
	return &base
}

func (s *Service) handleRedeem(response http.ResponseWriter, request *http.Request) {
	sessionData, ok := s.nonces.Take(request.PathValue("nonce"))
	if !ok {
		http.Error(response, "Launch URL is invalid or expired", http.StatusNotFound)
		return
	}
	s.sessionManager.Create(response, sessionData)
	redirectURL := s.frontendLandingUrl
	if taskResource := sessionData.GetByType("Task"); taskResource != nil {
		redirectURL = redirectURL.JoinPath("task", strings.Split(taskResource.Path, "/")[1])
	}
	http.Redirect(response, request, redirectURL.String(), http.StatusFound)
}

func (s *Service) authenticate(response http.ResponseWriter, request *http.Request) (*rp.TokenClaims, bool) {
	token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
	if token == "" || token == request.Header.Get("Authorization") {
		http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil, false
	}
	claims, err := s.tokenValidator.ValidateToken(request.Context(), token)
	if err != nil {
		slog.WarnContext(request.Context(), "Verification launch token rejected", slog.String(logging.FieldError, err.Error()))
		http.Error(response, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return nil, false
	}
	return claims, true
}

// buildSession mirrors what a real launch leaves behind: Patient, Practitioner, Organization,
// ServiceRequest and Condition, all cached in the session so the frontend needs no EHR. Everything is
// derived server-side from the Task — the caller only chooses which enrollment to launch.
func (s *Service) buildSession(ctx context.Context, tenant tenants.Properties, taskID string, practitionerName string) (*session.Data, error) {
	cpsFHIRClient, err := globals.CreateCPSFHIRClient(ctx)
	if err != nil {
		return nil, err
	}

	var task fhir.Task
	if err := cpsFHIRClient.ReadWithContext(ctx, "Task/"+taskID, &task); err != nil {
		return nil, fmt.Errorf("failed to read Task: %w", err)
	}
	if task.For == nil || task.For.Reference == nil || !strings.HasPrefix(*task.For.Reference, "Patient/") {
		return nil, fmt.Errorf("task %s has no patient reference", taskID)
	}

	var patient fhir.Patient
	if err := cpsFHIRClient.ReadWithContext(ctx, *task.For.Reference, &patient); err != nil {
		return nil, fmt.Errorf("failed to read Patient: %w", err)
	}

	sessionData := session.Data{
		FHIRLauncher: fhirLauncherKey,
		TenantID:     tenant.ID,
		LauncherProperties: map[string]string{
			"cps": tenant.URL(s.orcaPublicURL, careplanservice.FHIRBaseURL).String(),
		},
	}
	sessionData.Set(*task.For.Reference, patient)
	sessionData.Set("Task/"+taskID, task)

	practitioner := fhir.Practitioner{
		Id:   to.Ptr("magic-" + uuid.NewString()),
		Name: []fhir.HumanName{{Text: to.Ptr(practitionerName)}},
	}
	sessionData.Set("Practitioner/"+*practitioner.Id, practitioner)

	organizations, err := s.profile.Identities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to determine organization: %w", err)
	}
	if len(organizations) != 1 {
		return nil, fmt.Errorf("expected 1 organization, found %d", len(organizations))
	}
	sessionData.Set("Organization/magic-"+uuid.NewString(), organizations[0])

	if task.Focus != nil && task.Focus.Reference != nil && strings.HasPrefix(*task.Focus.Reference, "ServiceRequest/") {
		var serviceRequest fhir.ServiceRequest
		if err := cpsFHIRClient.ReadWithContext(ctx, *task.Focus.Reference, &serviceRequest); err == nil {
			sessionData.Set(*task.Focus.Reference, serviceRequest)
		}
	}

	if task.ReasonCode != nil && len(task.ReasonCode.Coding) > 0 {
		condition := fhir.Condition{
			Id:   to.Ptr("magic-" + uuid.NewString()),
			Code: task.ReasonCode,
		}
		sessionData.Set("Condition/"+*condition.Id, condition)
	}

	if len(task.Identifier) > 0 {
		sessionData.TaskIdentifier = to.Ptr(coolfhir.ToString(&task.Identifier[0]))
	}

	return &sessionData, nil
}
