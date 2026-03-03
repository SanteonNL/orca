package smartonfhir

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/cryptosigner"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/client/rp"
	zitadelHTTP "github.com/zitadel/oidc/v3/pkg/http"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"golang.org/x/oauth2"
)

const fhirLauncherKey = "smartonfhir"
const clientAssertionExpiry = 3 * time.Minute
const clockSkew = 5 * time.Second

const encounterSystem = "encounter"

func init() {
	// Register FHIR client factory that can create FHIR issuersByURL when the SMART on FHIR AppLaunch is used
	clients.Factories[fhirLauncherKey] = func(properties map[string]string) clients.ClientProperties {
		// TODO: Add context to the client creation?
		return clients.ClientProperties{
			Client:  createHTTPClient(context.Background(), properties["access_token"]).Transport,
			BaseURL: must.ParseURL(properties["iss"]),
		}
	}
}

type Service struct {
	config             Config
	tenants            tenants.Config
	sessionManager     *user.SessionManager[session.Data]
	frontendBaseURL    *url.URL
	orcaBaseURL        *url.URL
	issuersByURL       map[string]*trustedIssuer
	issuersByKey       map[string]*trustedIssuer
	strictMode         bool
	cookieHandler      *zitadelHTTP.CookieHandler
	jwtSigningKey      *jose.SigningKey
	jwtSigingKeyJWKSet *jose.JSONWebKeySet
	profile            profile.Provider
}

func (s *Service) CreateEHRProxies() (map[string]coolfhir.HttpProxy, map[string]fhirclient.Client) {
	// Currently not supported
	return map[string]coolfhir.HttpProxy{}, map[string]fhirclient.Client{}
}

type trustedIssuer struct {
	issuerLaunchURL       string
	mux                   *sync.RWMutex
	client                rp.RelyingParty
	key                   string
	clientID              string
	realIssuerURL         string
    fhirURL               string
	tenantID              string
	smartOnFhirHttpClient *http.Client
}

func (t trustedIssuer) issuerURL() string {
	// Epic's SMART on FHIR implementation uses an issuer URL that differs from the 'iss' parameter in the application launch,
	// so we override the 'iss' URL from launch with the configured URL.
	if t.realIssuerURL != "" {
		return t.realIssuerURL
	}
	return t.issuerLaunchURL
}

func New(config Config, tenants tenants.Config, sessionManager *user.SessionManager[session.Data], orcaBaseURL *url.URL, frontendBaseURL *url.URL, strictMode bool, profile profile.Provider) (*Service, error) {
	keysClient, err := azkeyvault.NewKeysClient(config.AzureKeyVault.URL, config.AzureKeyVault.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	certsClient, err := azkeyvault.NewCertificatesClient(config.AzureKeyVault.URL, config.AzureKeyVault.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}

	issuersByURL := make(map[string]*trustedIssuer)
	issuersByKey := make(map[string]*trustedIssuer)
	for key, curr := range config.Issuer {
		var tlsClientCert *tls.Certificate = nil
		if curr.ClientCertName != "" {
			tlsClientCert, err = azkeyvault.GetTLSCertificate(context.Background(), certsClient, keysClient, curr.ClientCertName)
			if err != nil {
				return nil, fmt.Errorf("unable to get TLS client certificate from Azure Key Vault: %w", err)
			}
		} else if curr.ClientCertFile != "" {
			certFileContents, err := os.ReadFile(curr.ClientCertFile)
			if err != nil {
				return nil, fmt.Errorf("unable to read TLS client certificate from file: %w", err)
			}
			tlsClientCertPtr, err := tls.X509KeyPair(certFileContents, certFileContents)
			if err != nil {
				return nil, fmt.Errorf("unable to create TLS client certificate: %w", err)
			}
			tlsClientCert = &tlsClientCertPtr
		}

		var httpClient *http.Client
		if tlsClientCert != nil {
			httpClient = &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						Certificates:  []tls.Certificate{*tlsClientCert},
						MinVersion:    tls.VersionTLS12,
						Renegotiation: tls.RenegotiateOnceAsClient,
					},
				},
			}
		} else {
			httpClient = &http.Client{
				Transport: http.DefaultTransport,
			}
		}

		issuer := &trustedIssuer{
			mux:                   &sync.RWMutex{},
			key:                   key,
			issuerLaunchURL:       curr.URL,
			clientID:              curr.ClientID,
			realIssuerURL:         curr.OAuth2URL,
			fhirURL:               curr.FHIRURL,
			tenantID:              curr.Tenant,
			smartOnFhirHttpClient: httpClient,
		}
		issuersByURL[curr.URL] = issuer
		issuersByKey[key] = issuer
	}
	cookieHashKey := make([]byte, 32)
	if _, err := rand.Read(cookieHashKey); err != nil {
		// Can't happen, but just in case (we don't want to end up with a zeroed key)
		panic(err)
	}
	cookieEncryptKey := make([]byte, 32)
	if _, err := rand.Read(cookieEncryptKey); err != nil {
		// Can't happen, but just in case (we don't want to end up with a zeroed key)
		panic(err)
	}
	var cookieHandlerOpts []zitadelHTTP.CookieHandlerOpt
	if !strictMode {
		cookieHandlerOpts = append(cookieHandlerOpts, zitadelHTTP.WithUnsecure())
	}
	//cookieHandlerOpts = append(cookieHandlerOpts, zitadelHTTP.WithSameSite(http.SameSiteNoneMode))
	service := &Service{
		config:          config,
		orcaBaseURL:     orcaBaseURL,
		frontendBaseURL: frontendBaseURL,
		strictMode:      strictMode,
		issuersByURL:    issuersByURL,
		issuersByKey:    issuersByKey,
		cookieHandler:   zitadelHTTP.NewCookieHandler(cookieHashKey, cookieEncryptKey, cookieHandlerOpts...),
		sessionManager:  sessionManager,
		tenants:         tenants,
		profile:         profile,
	}
	if len(config.AzureKeyVault.SigningKeyName) > 0 {
		var err error
		service.jwtSigningKey, service.jwtSigingKeyJWKSet, err = loadJWTSigningKeyFromAzureKeyVault(config.AzureKeyVault, strictMode)
		if err != nil {
			return nil, fmt.Errorf("failed to load JWK set for JWT signing from Azure Key Vault: %w", err)
		}
	} else {
		slog.Info("SMART on FHIR: no Azure Key Vault configured for JWT signing, generating an in-memory key")
		privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		service.jwtSigningKey = &jose.SigningKey{
			Algorithm: jose.ES256,
			Key:       privateKey,
		}
		service.jwtSigingKeyJWKSet = &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:   privateKey.Public(),
					KeyID: "default",
					Use:   "sig",
				},
			},
		}
	}
	return service, nil
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /smart-app-launch", s.handleAppLaunch)
	mux.HandleFunc("GET /smart-app-launch/callback/{key}", s.handleCallback)
	mux.HandleFunc("GET /smart-app-launch/.well-known/jwks.json", s.handleGetJWKs)
}

func (s *Service) handleAppLaunch(response http.ResponseWriter, request *http.Request) {
	// TODO: check whether the issuer is trusted
	issuer := request.URL.Query().Get("iss")
	slog.InfoContext(request.Context(), "SMART on FHIR app launch request", slog.String(logging.FieldUrl, request.URL.String()))
	if len(issuer) == 0 {
		s.SendError(request.Context(), issuer, errors.New("invalid iss parameter"), response, http.StatusBadRequest)
		return
	}
	// TODO: check if 'launch' is needed
	launch := request.URL.Query().Get("launch")

	provider, err := s.getIssuerByURL(request, issuer)
	if err != nil {
		s.SendError(request.Context(), issuer, fmt.Errorf("failed to get OIDC client for issuer: %w", err), response, http.StatusInternalServerError)
		return
	}
	urlOptions := []rp.URLParamOpt{}
	if launch != "" {
		urlOptions = append(urlOptions, rp.WithURLParam("launch", launch))
	}
	// Epic on FHIR requirement: aud claim in the authorization request
	urlOptions = append(urlOptions, rp.WithURLParam("aud", provider.Issuer()))
	var stateParams url.Values
	for key, value := range request.URL.Query() {
		switch key {
		case "iss", "launch":
			continue
		default:
			stateParams[key] = value
		}
	}
	rp.AuthURLHandler(
		func() string {
			return hex.EncodeToString([]byte(stateParams.Encode()))
		},
		provider,
		urlOptions...,
	)(response, request)
}

func (s *Service) handleCallback(response http.ResponseWriter, request *http.Request) {
	issuerKey := request.PathValue("key")
	issuer, ok := s.issuersByKey[issuerKey]
	if !ok {
		s.SendError(request.Context(), "key: "+issuerKey, fmt.Errorf("unknown issuer key: %s", issuerKey), response, http.StatusBadRequest)
		return
	}
	// zitadel/oidc's client_assertion JWT profile doesn't support the 'jti' parameter,
	// so we sign it ourselves and pass it as a URL parameter.
	clientAssertion, err := s.createClientAssertion(issuer)
	if err != nil {
		s.SendError(request.Context(), issuer.key, fmt.Errorf("failed to create client assertion: %w", err), response, http.StatusInternalServerError)
		return
	}
	var codeExchangeOpts = []rp.URLParamOpt{
		rp.URLParamOpt(rp.WithClientAssertionJWT(clientAssertion)),
	}
	rp.CodeExchangeHandler(func(httpResponse http.ResponseWriter, httpRequest *http.Request, tokens *oidc.Tokens[*oidc.IDTokenClaims], state string, rp rp.RelyingParty) {
		idTokenJSON, _ := json.Marshal(tokens.IDTokenClaims)
		slog.DebugContext(
			request.Context(),
			"SMART on FHIR app launched with ID token",
			slog.String("token", string(idTokenJSON)),
		)
		ctx := httpRequest.Context()
		context, err := s.loadContext(ctx, issuer, tokens)
		if err != nil {
			s.SendError(request.Context(), issuer.key, fmt.Errorf("failed to load context for SMART App Launch: %w", err), httpResponse, http.StatusInternalServerError)
			return
		}
		// destructure context
		patient := context.Patient
		practitioner := context.Practitioner
		encounter := context.Encounter
		existingTask := context.ExistingTask
		organization := context.Organization
		tenant := context.Properties

		// Get bsn from patient identifier using system 2.16.840.1.113883.2.4.6.3 or http://fhir.nl/fhir/NamingSystem/bsn
		var bsn string
		for _, identifier := range patient.Identifier {
			if identifier.System != nil && (strings.EqualFold(*identifier.System, "2.16.840.1.113883.2.4.6.3") || strings.EqualFold(*identifier.System, "http://fhir.nl/fhir/NamingSystem/bsn")) {
				bsn = *identifier.Value
				break
			}
		}
		if bsn == "" {
			s.SendError(request.Context(), issuer.key, fmt.Errorf("no BSN found for patient in SMART App Launch"), httpResponse, http.StatusBadRequest)
			return
		}

		// TODO don't hardcode this.
		conditionCode := fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("84114007"),
					Display: to.Ptr("hartfalen"),
				},
			},
			Text: to.Ptr("hartfalen"),
		}

		condition := fhir.Condition{
			Id:   to.Ptr(uuid.NewString()),
			Code: to.Ptr(conditionCode),
		}

		taskPerformer := fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  &s.config.TaskPerformerUra,
			},
		}
		// Enrich performer URA with registered name
		if result, err := s.profile.CsdDirectory().LookupEntity(ctx, *taskPerformer.Identifier); err != nil {
			slog.WarnContext(ctx, "Couldn't resolve performer name", slog.String("ura", s.config.TaskPerformerUra), slog.String(logging.FieldError, err.Error()))
		} else {
			taskPerformer = *result
		}

		var serviceRequest *fhir.ServiceRequest
		if encounter != nil {
			serviceRequest = &fhir.ServiceRequest{
				Status: fhir.RequestStatusActive,
				Identifier: []fhir.Identifier{
					{
						System: to.Ptr(encounterSystem),
						Value:  to.Ptr(*encounter.Id),
					},
				},
				Code: &fhir.CodeableConcept{
					Coding: []fhir.Coding{
						// Hardcoded, we only do Telemonitoring for now
						{
							System:  to.Ptr("http://snomed.info/sct"),
							Code:    to.Ptr("719858009"),
							Display: to.Ptr("Thuismonitoring"),
						},
					},
				},
				ReasonReference: []fhir.Reference{{
					Type:      to.Ptr("Condition"),
					Reference: to.Ptr("Condition/magic-" + *condition.Id),
					Display:   to.Ptr(*condition.Code.Text),
				}},
				Subject: fhir.Reference{
					Type: to.Ptr("Patient"),
					Identifier: &fhir.Identifier{
						System: to.Ptr(coolfhir.BSNNamingSystem),
						Value:  &bsn,
					},
				},
				Performer: []fhir.Reference{taskPerformer},
				Requester: &fhir.Reference{
					Identifier: &organization.Identifier[0],
				},
			}
		}

		sessionData := session.Data{
			FHIRLauncher: fhirLauncherKey,
			LauncherProperties: map[string]string{
				"access_token": tokens.AccessToken,
				"iss":          issuer.issuerURL(),
			},
			TenantID: tenant.ID,
		}
		if encounter != nil {
			sessionData.TaskIdentifier = to.Ptr(encounterSystem + "|" + *encounter.Id)
		}

		sessionData.Set("Patient/"+*patient.Id, *patient)
		sessionData.Set("Practitioner/"+*practitioner.Id, *practitioner)
		if serviceRequest != nil {
			sessionData.Set("ServiceRequest/magic-"+uuid.NewString(), *serviceRequest)
		}
		sessionData.Set("Organization/magic-"+uuid.NewString(), *organization)
		sessionData.Set("Condition/magic-"+*condition.Id, condition)
		if existingTask != nil {
			sessionData.Set(*to.Ptr("Task/" + *existingTask.Id), *existingTask)
		}
		// TODO opt sessionData.Set("PractitionerRole/magic-"+uuid.NewString(), launchContext.PractitionerRole)

		s.sessionManager.Create(httpResponse, sessionData)
		slog.InfoContext(request.Context(), "SMART on FHIR app launch succeeded")

		var redirectURL *url.URL

		if encounter == nil {
			// App launch without encounter context, redirect to list page.
			redirectURL = s.frontendBaseURL.JoinPath("list")
		} else if existingTask == nil {
			redirectURL = s.frontendBaseURL.JoinPath("new")
		} else {
			redirectURL = s.frontendBaseURL.JoinPath("task", *existingTask.Id)
		}

		http.Redirect(httpResponse, request, redirectURL.String(), http.StatusFound)
	}, issuer.client, codeExchangeOpts...)(response, request)
}

// struct for return values of method below
type contextData struct {
	Patient      *fhir.Patient
	Practitioner *fhir.Practitioner
	Encounter    *fhir.Encounter
	Organization *fhir.Organization
	ExistingTask *fhir.Task
	Properties   *tenants.Properties
}

func (s *Service) loadContext(ctx context.Context, issuer *trustedIssuer, tokens *oidc.Tokens[*oidc.IDTokenClaims]) (*contextData, error) {
	// Select tenant
	tenant, err := s.tenants.Get(issuer.tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %s: %w", issuer.tenantID, err)
	}
	ctx = tenants.WithTenant(ctx, *tenant)

	patientID, hasPatientID := tokens.Extra("patient").(string)
	if !hasPatientID || patientID == "" {
		return nil, fmt.Errorf("no patient ID found in token response")
	}

	encounterID, hasEncounterID := tokens.Extra("encounter").(string)
	if !hasEncounterID || encounterID == "" {
		slog.InfoContext(
			ctx,
			"No encounter ID found in token response, proceeding without encounter context")
	}

	var fhirUrl string
	if issuer.fhirURL != "" {
		fhirUrl = issuer.fhirURL
	} else {
		fhirUrl = issuer.issuerLaunchURL
	}
	apiUrl, err := url.Parse(fhirUrl)
	if err != nil {
		return nil, fmt.Errorf("could not parse API URL from config: %w", err)
	}

	fhirClient := fhirclient.New(apiUrl, issuer.createSmartOnFhirApiClient(tokens.AccessToken), coolfhir.Config())
	cpsFHIRClient, err := globals.CreateCPSFHIRClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to create CPS FHIR client: %w", err)
	}

	fhirUserClaim, hasFhirUserClaim := tokens.IDTokenClaims.Claims["fhirUser"].(string)
	if !hasFhirUserClaim || fhirUserClaim == "" {
		return nil, fmt.Errorf("no fhirUser claim found in token response")
	}
	fhirUserClaimParts := strings.Split(fhirUserClaim, "/")
	practitionerId := fhirUserClaimParts[len(fhirUserClaimParts)-1]
	var practitioner fhir.Practitioner
	if err = fhirClient.Read("Practitioner/"+practitionerId, &practitioner); err != nil {
		return nil, fmt.Errorf("unable to fetch Practitioner bundle: %w", err)
	}

	slog.DebugContext(
		ctx,
		"SMART on FHIR practitioner",
		slog.String("practitioner_id", *practitioner.Id),
	)

	var patient fhir.Patient
	if err = fhirClient.Read("Patient/"+patientID, &patient); err != nil {
		return nil, fmt.Errorf("failed to read Patient resource: %w", err)
	}

	var encounter *fhir.Encounter
	var existingTask *fhir.Task
	if hasEncounterID && encounterID != "" {
		encounter = &fhir.Encounter{
			Id: to.Ptr(encounterID),
		}
		var errTask error
		existingTask, errTask = coolfhir.GetTaskByIdentifier(ctx, cpsFHIRClient, fhir.Identifier{
			System: to.Ptr(encounterSystem),
			Value:  to.Ptr(*encounter.Id),
		})
		if errTask != nil {
			return nil, fmt.Errorf("failed to check for existing CPS Task resource:%w", errTask)
		}
	}

	// Resolve identity of local care organization
	identities, err := s.profile.Identities(ctx)
	if err != nil {
	}
	if len(identities) != 1 {
		return nil, fmt.Errorf("expected exactly one identity, got %d", len(identities))
	}
	organization := identities[0]

	if !s.strictMode {
		MakePatientCompatible(ctx, &patient)
	}

	return &contextData{
		Patient:      &patient,
		Practitioner: &practitioner,
		Encounter:    encounter,
		ExistingTask: existingTask,
		Organization: &organization,
		Properties:   tenant,
	}, nil
}
func (s *Service) getIssuerByKey(request *http.Request, issuerKey string) (rp.RelyingParty, error) {
	iss, ok := s.issuersByKey[issuerKey]
	if !ok {
		return nil, errors.New("unknown SMART on FHIR issuer")
	}
	return s.initializeIssuer(request.Context(), iss)
}

func (s *Service) getIssuerByURL(request *http.Request, issuer string) (rp.RelyingParty, error) {
	iss, ok := s.issuersByURL[issuer]
	if !ok {
		return nil, errors.New("unknown SMART on FHIR issuer")
	}
	return s.initializeIssuer(request.Context(), iss)
}

func (s *Service) initializeIssuer(ctx context.Context, issuer *trustedIssuer) (rp.RelyingParty, error) {
	issuer.mux.RLock()
	if issuer.client != nil {
		issuer.mux.RUnlock()
		return issuer.client, nil
	}
	issuer.mux.RUnlock()
	// Client not created yet
	issuer.mux.Lock()
	defer issuer.mux.Unlock()

	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}),
	)
	options := []rp.Option{
		rp.WithCookieHandler(s.cookieHandler),
		rp.WithVerifierOpts(rp.WithIssuedAtOffset(clockSkew)),
		rp.WithHTTPClient(otel.NewTracedHTTPClient("smartonfhir")),
		rp.WithSigningAlgsFromDiscovery(),
		rp.WithLogger(logger),
		rp.WithUnauthorizedHandler(func(httpResponse http.ResponseWriter, httpRequest *http.Request, desc string, _ string) {
			s.SendError(httpRequest.Context(), issuer.key, fmt.Errorf("unauthorized: %s", desc), httpResponse, http.StatusUnauthorized)
		}),
	}

	scopes := []string{"openid", "fhirUser", "launch"}
	redirectURI := s.orcaBaseURL.JoinPath("smart-app-launch", "callback", issuer.key)
	slog.InfoContext(
		ctx,
		"Initiating SMART on FHIR flow",
		slog.String("issuer_url", issuer.issuerURL()),
		slog.String("client_id", issuer.clientID),
		slog.String("redirect_uri", redirectURI.String()),
		slog.String("scopes", strings.Join(scopes, ",")),
	)
	provider, err := rp.NewRelyingPartyOIDC(ctx, issuer.issuerURL(), issuer.clientID, "client_secret_todo", redirectURI.String(), scopes, options...)
	if err != nil {
		return nil, fmt.Errorf("provider: %w", err)
	}
	// Store the client in the map
	issuer.client = provider
	return provider, nil
}

func (s *Service) handleGetJWKs(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	jsonBytes, _ := json.Marshal(s.jwtSigingKeyJWKSet)
	httpResponse.Header().Set("Content-Type", "application/json")
	httpResponse.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour, seems like a sensible default
	httpResponse.WriteHeader(http.StatusOK)
	_, err := httpResponse.Write(jsonBytes)
	if err != nil {
		slog.WarnContext(httpRequest.Context(), "Failed to write JWKSet response", slog.String(logging.FieldError, err.Error()))
		return
	}
}

func (s *Service) SendError(ctx context.Context, issuer string, err error, httpResponse http.ResponseWriter, httpStatusCode int) {
	launchId := uuid.NewString()
	slog.ErrorContext(
		ctx,
		"SMART on FHIR launch failed",
		slog.String("issuer", issuer),
		slog.String("launch_id", launchId),
		slog.String(logging.FieldError, err.Error()),
	)
	// TODO: nice error page
	msg := "SMART on FHIR launch failed (id=" + launchId + ")"
	if !s.strictMode {
		msg += ": " + err.Error()
	}
	slog.ErrorContext(
		ctx,
		"HTTP error response sent for SMART on FHIR launch failure",
		slog.String("issuer", issuer),
		slog.String("launch_id", launchId),
		slog.String(logging.FieldError, err.Error()),
		slog.Int("http_status_code", httpStatusCode),
		slog.String("msg", msg),
	)
	http.Error(httpResponse, http.StatusText(httpStatusCode), httpStatusCode)
}

func (s *Service) createClientAssertion(issuer *trustedIssuer) (string, error) {
	return s.createClientAssertionForAudience(issuer.clientID, issuer.client.OAuthConfig().Endpoint.TokenURL)
}

func (s *Service) createClientAssertionForAudience(clientID string, audience string) (string, error) {
	signer, err := jose.NewSigner(*s.jwtSigningKey, (&jose.SignerOptions{}).
		WithType("JWT").
		WithHeader("kid", s.jwtSigingKeyJWKSet.Keys[0].KeyID))
	if err != nil {
		return "", fmt.Errorf("failed to create JWT signer: %w", err)
	}
	cl := jwt.Claims{
		Subject:   clientID,
		Issuer:    clientID,
		NotBefore: jwt.NewNumericDate(time.Now().Add(-clockSkew)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Expiry:    jwt.NewNumericDate(time.Now().Add(clientAssertionExpiry)),
		Audience:  jwt.Audience{audience},
		ID:        uuid.NewString(),
	}
	result, err := jwt.Signed(signer).Claims(cl).Serialize()
	if err != nil {
		return "", fmt.Errorf("failed to serialize JWT client assertion: %w", err)
	}
	if !s.strictMode {
		slog.Debug("Created JWT client assertion", slog.String("assertion", result))
	}
	return result, nil
}

func createHTTPClient(ctx context.Context, accessToken string) *http.Client {
	// TODO: take expiry into account?
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}))
}

func createFHIRClient(ctx context.Context, baseURL *url.URL, accessToken string) fhirclient.Client {
	// Create a FHIR client with the provided base URL and access token
	httpClient := createHTTPClient(ctx, accessToken)
	return fhirclient.New(baseURL, httpClient, coolfhir.Config())
}

func loadJWTSigningKeyFromAzureKeyVault(config AzureKeyVaultConfig, strictMode bool) (*jose.SigningKey, *jose.JSONWebKeySet, error) {
	keysClient, err := azkeyvault.NewKeysClient(config.URL, config.CredentialType, !strictMode)
	if err != nil {
		return nil, nil, err
	}
	key, err := azkeyvault.GetKey(keysClient, config.SigningKeyName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get key (name: %s): %w", config.SigningKeyName, err)
	}

	// Use thumbprint as key ID, to avoid leaking Azure network information through the key ID
	keyID := hex.EncodeToString(key.PublicKeyThumbprintS256())

	// Log this for analysis: key ID can't be related back to a specific Azure Key Vault version (because it's a thumbprint),
	// so we log it to be able to trace back which key was used in case of issues.
	slog.Info(
		"Loaded SMART on FHIR JWT signing key from Azure Key Vault",
		slog.String("key_name", key.KeyName()),
		slog.String("key_version", key.KeyVersion()),
		slog.String("jwk_key_id", keyID),
	)

	return &jose.SigningKey{
			Algorithm: jose.SignatureAlgorithm(key.SigningAlgorithm()),
			Key:       cryptosigner.Opaque(key),
		}, &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:       key.Public(),
					KeyID:     keyID,
					Use:       "sig",
					Algorithm: key.SigningAlgorithm(),
				},
			},
		}, nil
}

func (s *trustedIssuer) createSmartOnFhirApiClient(accessToken string) *http.Client {
	return &http.Client{
		Transport: &smartOnFhirRoundTripper{
			value: "Bearer " + accessToken,
			inner: s.smartOnFhirHttpClient.Transport,
		},
	}
}

type smartOnFhirRoundTripper struct {
	value string
	inner http.RoundTripper
}

func (a smartOnFhirRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Add("Accept", "application/fhir+json")
	request.Header.Add("Authorization", a.value)
	return a.inner.RoundTrip(request)
}

func MakePatientCompatible(ctx context.Context, patient *fhir.Patient) {
	hasBsn := false
	for _, identifier := range patient.Identifier {
		if identifier.System != nil && (strings.EqualFold(*identifier.System, "2.16.840.1.113883.2.4.6.3") || strings.EqualFold(*identifier.System, "http://fhir.nl/fhir/NamingSystem/bsn")) {
			hasBsn = true
			break
		}
	}
	if !hasBsn {
		bsn := generateValidBSN()
		// add bsn to patient struct for use in the app
		patient.Identifier = append(patient.Identifier, fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
			Value:  &bsn,
		})
		slog.WarnContext(ctx, "No BSN found for patient, generated valid fake BSN for use in SMART App Launch", slog.String("generated_bsn", bsn))

	}

	hasPhone := false
	for _, telecom := range patient.Telecom {
		if *telecom.System == fhir.ContactPointSystemPhone && telecom.Value != nil && strings.HasPrefix(*telecom.Value, "06") {
			hasPhone = true
			break
		}
	}
	if !hasPhone {
		fakePhone := "06" + fmt.Sprintf("%08d", mrand.Intn(100000000))
		patient.Telecom = append(patient.Telecom, fhir.ContactPoint{
			System: to.Ptr(fhir.ContactPointSystemPhone),
			Value:  &fakePhone,
		})
		slog.WarnContext(ctx, "No mobile phone number found for patient, generated fake phone number for use in SMART App Launch", slog.String("generated_phone", fakePhone))
	}

	hasEmail := false
	for _, telecom := range patient.Telecom {
		if *telecom.System == fhir.ContactPointSystemEmail && telecom.Value != nil {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		fakeEmail := fmt.Sprintf("user%d@example.com", mrand.Intn(1000000))
		patient.Telecom = append(patient.Telecom, fhir.ContactPoint{
			System: to.Ptr(fhir.ContactPointSystemEmail),
			Value:  &fakeEmail,
		})
		slog.WarnContext(ctx, "No email found for patient, generated fake email for use in SMART App Launch", slog.String("generated_email", fakeEmail))
	}
}

// generateValidBSN generates a random valid Dutch BSN (Burger Service Nummer) using the 11-test (Elfproef)
func generateValidBSN() string {
	for {
		// Generate a random 8-digit number (since the 9th digit is the check digit)
		n := mrand.Intn(90000000) + 10000000 // ensures 8 digits, not starting with 0
		digits := make([]int, 9)
		for i := 7; i >= 0; i-- {
			digits[i] = n % 10
			n /= 10
		}
		// Calculate the check digit (Elfproef)
		sum := 0
		for i := 0; i < 8; i++ {
			sum += digits[i] * (9 - i)
		}
		for check := 0; check < 10; check++ {
			if (sum+(-1*check))%11 == 0 {
				digits[8] = check
				// Compose the BSN string
				bsn := 0
				for _, d := range digits {
					bsn = bsn*10 + d
				}
				// BSN must be between 100000000 and 999999999
				if bsn >= 100000000 && bsn <= 999999999 {
					return fmt.Sprintf("%09d", bsn)
				}
			}
		}
	}
}
