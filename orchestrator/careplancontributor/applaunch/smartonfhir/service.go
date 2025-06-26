package smartonfhir

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
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
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const fhirLauncherKey = "smartonfhir"
const clientAssertionExpiry = 5 * time.Minute
const clockSkew = 5 * time.Second

type IDTokenClaims struct {
	oidc.IDTokenClaims
	FHIRUser string `json:"fhirUser,omitempty"`
}

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
	sessionManager     *user.SessionManager[session.Data]
	frontendBaseURL    *url.URL
	orcaBaseURL        *url.URL
	issuersByURL       map[string]*trustedIssuer
	issuersByKey       map[string]*trustedIssuer
	strictMode         bool
	cookieHandler      *zitadelHTTP.CookieHandler
	jwtSigningKey      *jose.SigningKey
	jwtSigingKeyJWKSet *jose.JSONWebKeySet
}

type trustedIssuer struct {
	issuerLaunchURL string
	mux             *sync.RWMutex
	client          rp.RelyingParty
	key             string
	clientID        string
	realIssuerURL   string
}

func (t trustedIssuer) issuerURL() string {
	// Epic's SMART on FHIR implementation uses an issuer URL that differs from the 'iss' parameter in the application launch,
	// so we override the 'iss' URL from launch with the configured URL.
	if t.realIssuerURL != "" {
		return t.realIssuerURL
	}
	return t.issuerLaunchURL
}

func New(config Config, sessionManager *user.SessionManager[session.Data], orcaBaseURL *url.URL, frontendBaseURL *url.URL, strictMode bool) (*Service, error) {
	issuersByURL := make(map[string]*trustedIssuer)
	issuersByKey := make(map[string]*trustedIssuer)
	for _, curr := range config.Issuer {
		issuerKeyBytes := md5.Sum([]byte(curr.URL))
		issuerKey := hex.EncodeToString(issuerKeyBytes[:])
		issuer := &trustedIssuer{
			mux:             &sync.RWMutex{},
			key:             issuerKey,
			issuerLaunchURL: curr.URL,
			clientID:        curr.ClientID,
			realIssuerURL:   curr.DiscoveryURL,
		}
		issuersByURL[curr.URL] = issuer
		issuersByKey[issuerKey] = issuer
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
	}
	if len(config.AzureKeyVault.SigningKeyName) > 0 {
		var err error
		service.jwtSigningKey, service.jwtSigingKeyJWKSet, err = loadJWTSigningKeyFromAzureKeyVault(config.AzureKeyVault, strictMode)
		if err != nil {
			return nil, fmt.Errorf("failed to load JWK set for JWT signing from Azure Key Vault: %w", err)
		}
	} else {
		log.Info().Msg("SMART on FHIR: no Azure Key Vault configured for JWT signing, generating an in-memory key")
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
	if len(issuer) == 0 {
		http.Error(response, "invalid iss parameter", http.StatusBadRequest)
		return
	}
	// TODO: check if 'launch' is needed
	launch := request.URL.Query().Get("launch")

	provider, err := s.getIssuerByURL(request, issuer)
	if err != nil {
		// TODO: make this a nicer error page
		log.Ctx(request.Context()).Err(err).Msg("Failed to get OIDC client for issuer")
		http.Error(response, clientError("SMART on FHIR initialization error", err, s.strictMode), http.StatusServiceUnavailable)
		return
	}
	urlOptions := []rp.URLParamOpt{}
	if launch != "" {
		urlOptions = append(urlOptions, rp.WithURLParam("launch", launch))
	}
	// TODO: is this required?
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
		// TODO: make this a nicer error page
		log.Ctx(request.Context()).Error().Msgf("Unknown issuer key: %s", issuerKey)
		http.Error(response, "Unknown issuer", http.StatusBadRequest)
		return
	}
	// zitadel/oidc's client_assertion JWT profile doesn't support the 'jti' parameter,
	// so we sign it ourselves and pass it as a URL parameter.
	clientAssertion, err := s.createClientAssertion(issuer)
	if err != nil {
		// TODO: make this a nicer error page
		log.Ctx(request.Context()).Error().Msgf("Failed to create client assertion for issuer %s: %v", issuerKey, err)
		http.Error(response, clientError("Internal error", err, s.strictMode), http.StatusInternalServerError)
		return
	}
	var codeExchangeOpts = []rp.URLParamOpt{
		rp.URLParamOpt(rp.WithClientAssertionJWT(clientAssertion)),
	}
	rp.CodeExchangeHandler(func(httpResponse http.ResponseWriter, httpRequest *http.Request, tokens *oidc.Tokens[*IDTokenClaims], state string, rp rp.RelyingParty) {
		idTokenJSON, _ := json.Marshal(tokens.IDTokenClaims)
		log.Ctx(httpRequest.Context()).Info().Msgf("SMART on FHIR app launched with ID token: %s", idTokenJSON)
		patient, practitioner, err := s.loadContext(httpRequest.Context(), issuer, tokens)
		if err != nil {
			// TODO: make this a nicer error page
			log.Ctx(httpRequest.Context()).Error().Err(err).Msg("Failed to load context for SMART App Launch")
			http.Error(httpResponse, clientError("Failed to load context", err, s.strictMode), http.StatusInternalServerError)
			return
		}
		sessionData := session.Data{
			FHIRLauncher: fhirLauncherKey,
			LauncherProperties: map[string]string{
				"access_token": tokens.AccessToken,
				"iss":          issuer.issuerURL(),
			},
		}
		sessionData.Set("Patient/"+*patient.Id, *patient)
		sessionData.Set("Practitioner/"+*practitioner.Id, *practitioner)
		s.sessionManager.Create(httpResponse, sessionData)
		log.Ctx(request.Context()).Info().Msg("SMART on FHIR app launch succeeded")
		// Note: we don't support enrolment/task creation through SMART on FHIR yet, so we redirect to the task overview
		http.Redirect(httpResponse, request, s.frontendBaseURL.JoinPath("list").String(), http.StatusFound)
	}, issuer.client, codeExchangeOpts...)(response, request)
}

func (s *Service) loadContext(ctx context.Context, issuer *trustedIssuer, tokens *oidc.Tokens[*IDTokenClaims]) (*fhir.Patient, *fhir.Practitioner, error) {
	fhirClient := createFHIRClient(ctx, must.ParseURL(issuer.issuerLaunchURL), tokens.AccessToken)
	var patient fhir.Patient
	patientID, hasPatientID := tokens.Extra("patient").(string)
	if !hasPatientID || patientID == "" {
		return nil, nil, errors.New("patient ID not found in ID token claims")
	}
	if err := fhirClient.Read("Patient/"+patientID, &patient); err != nil {
		return nil, nil, fmt.Errorf("failed to read patient resource: %w", err)
	}
	if tokens.IDTokenClaims.FHIRUser == "" {
		return nil, nil, errors.New("fhirUser claim (practitioner) not found in id_token claims")
	}
	// fhirUser claim can contain either a relative URL (SMART on FHIR Sandbox Launcher), or an absolute URL (Epic Sandbox).
	// We assume, it's always a FHIR Practitioner resource.
	var practitioner fhir.Practitioner
	if err := fhirClient.Read(tokens.IDTokenClaims.FHIRUser, &practitioner); err != nil {
		return nil, nil, fmt.Errorf("failed to read practitioner resource: %w", err)
	}
	return &patient, &practitioner, nil
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
		rp.WithHTTPClient(http.DefaultClient),
		rp.WithSigningAlgsFromDiscovery(),
		rp.WithLogger(logger),
	}

	scopes := []string{"openid", "fhirUser", "user/Patient.r", "user/Practitioner.r", "launch"}
	redirectURI := s.orcaBaseURL.JoinPath("smart-app-launch", "callback", issuer.key)
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
		log.Ctx(httpRequest.Context()).Warn().Err(err).Msg("Failed to write JWKSet response")
		return
	}
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
	return result, nil
}

func clientError(msg string, err error, strictMode bool) string {
	if strictMode {
		return msg
	}
	// In non-strict mode, we also return the underlying error
	return fmt.Sprintf("%s: %v", msg, err)
}

func createHTTPClient(ctx context.Context, accessToken string) *http.Client {
	// TODO: create http.Client that adds the access token to the Authorization header
	// TODO: take expiry into account?
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}))
}

func createFHIRClient(ctx context.Context, baseURL *url.URL, accessToken string) fhirclient.Client {
	// Create a FHIR client with the provided base URL and access token
	httpClient := createHTTPClient(ctx, accessToken)
	return fhirclient.New(baseURL, httpClient, nil)
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
	return &jose.SigningKey{
			Algorithm: jose.SignatureAlgorithm(key.SigningAlgorithm()),
			Key:       cryptosigner.Opaque(key),
		}, &jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:   key.Public(),
					KeyID: key.KeyID(),
					Use:   "sig",
				},
			},
		}, nil
}
