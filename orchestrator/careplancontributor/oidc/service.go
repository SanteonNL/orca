package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zitadel/oidc/v3/pkg/oidc"
	"github.com/zitadel/oidc/v3/pkg/op"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"golang.org/x/text/language"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
)

type Service struct {
	provider        op.OpenIDProvider
	callbackURLFunc func(context.Context, string) string
	storage         *Storage
	issuerURL       *url.URL
}

func New(strictMode bool, issuer *url.URL, config Config) (*Service, error) {
	log.Info().Msg("Initializing OpenID Connect Provider")
	var extraOptions []op.Option
	if strictMode {
		extraOptions = append(extraOptions, op.WithAllowInsecure())
	}
	// TODO: Change to key in Azure Key Vault
	key := [32]byte{}
	_, _ = rand.Read(key[:])

	// TODO: Change to key in Azure Key Vault
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}
	keyID := uuid.NewString()
	signingKey := SigningKey{
		id:           keyID,
		sigAlgorithm: "RS256",
		key:          privateKey,
	}

	storage := &Storage{
		mux:          &sync.RWMutex{},
		authRequests: make(map[string]AuthRequest),
		tokens:       make(map[string]Token),
		signingKey:   signingKey,
		clients:      make(map[string]op.Client),
	}
	for _, client := range config.Clients {
		if _, exists := storage.clients[client.ID]; exists {
			return nil, fmt.Errorf("duplicate client_id: %s", client.ID)
		}
		storage.clients[client.ID] = Client{
			id:           client.ID,
			redirectURIs: []string{client.RedirectURI},
			loginURL:     issuer.JoinPath("login"),
		}
	}

	provider, err := newOIDCProvider(storage, issuer.String(), key, slog.Default(), extraOptions)
	if err != nil {
		return nil, err
	}
	return &Service{
		issuerURL:       issuer,
		provider:        provider,
		storage:         storage,
		callbackURLFunc: op.AuthCallbackURL(provider),
	}, nil
}

func (s *Service) HandleLogin(httpResponse http.ResponseWriter, httpRequest *http.Request, session *user.SessionData) {
	ctx := op.ContextWithIssuer(httpRequest.Context(), s.issuerURL.String())
	// TODO: Prevent CSRF
	if err := httpRequest.ParseForm(); err != nil {
		http.Error(httpResponse, fmt.Errorf("parse form: %w", err).Error(), http.StatusBadRequest)
		return
	}
	authRequestID := httpRequest.FormValue("authRequestID") // specified by Zitadel/OpenID Provider
	if authRequestID == "" {
		http.Error(httpResponse, "authRequestID is required", http.StatusBadRequest)
		return
	}
	userDetails, err := userFromSession(session)
	if err != nil {
		http.Error(httpResponse, fmt.Errorf("get user from session: %w", err).Error(), http.StatusBadRequest)
		return
	}
	if err = s.storage.AuthenticateUser(ctx, authRequestID, *userDetails); err != nil {
		http.Error(httpResponse, fmt.Errorf("authenticate user: %w", err).Error(), http.StatusInternalServerError)
		return
	}
	redirectURL := s.callbackURLFunc(ctx, authRequestID)
	http.Redirect(httpResponse, httpRequest, redirectURL, http.StatusFound)
}

func userFromSession(session *user.SessionData) (*UserDetails, error) {
	var userDetails UserDetails
	// Get practitioner from session
	practitionerRef := session.StringValues["practitioner"] // reference to the FHIR practitioner resource
	if practitionerRef == "" {
		return nil, errors.New("no practitioner reference in session")
	}
	if session.OtherValues == nil || session.OtherValues[practitionerRef] == nil {
		return nil, errors.New("practitioner object not found in session")
	}
	practitioner, ok := session.OtherValues[practitionerRef].(fhir.Practitioner)
	if !ok {
		return nil, fmt.Errorf("practitioner object in session should be fhir.Practitioner, but was %T", session.OtherValues[practitionerRef])
	}
	for _, name := range practitioner.Name {
		userDetails.Name = coolfhir.FormatHumanName(name)
		break
	}
	for _, identifier := range practitioner.Identifier {
		userDetails.ID = *identifier.Value
	}
	if userDetails.ID == "" {
		return nil, errors.New("no identifier found in practitioner resource")
	}
	for _, qualification := range practitioner.Qualification {
		for _, coding := range qualification.Code.Coding {
			userDetails.Roles = append(userDetails.Roles, to.EmptyString(coding.Code)+"@"+to.EmptyString(coding.System))
		}
	}
	for _, contactPoint := range practitioner.Telecom {
		if contactPoint.System != nil && *contactPoint.System == fhir.ContactPointSystemEmail &&
			contactPoint.Value != nil {
			userDetails.Email = *contactPoint.Value
			break
		}
	}
	// Get patient from session
	patientRef := session.StringValues["patient"]
	if patientRef == "" {
		return nil, errors.New("no patient reference in session")
	}
	patient, ok := session.OtherValues[patientRef].(fhir.Patient)
	if !ok {
		return nil, fmt.Errorf("patient object in session should be fhir.Patient, but was %T", session.OtherValues[patientRef])
	}
	for _, identifier := range patient.Identifier {
		userDetails.PatientIdentifiers = append(userDetails.PatientIdentifiers, identifier)
	}
	return &userDetails, nil
}

func (s *Service) ServeHTTP(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	s.provider.ServeHTTP(httpResponse, httpRequest)
}

func newOIDCProvider(storage op.Storage, issuer string, key [32]byte, logger *slog.Logger, extraOptions []op.Option) (op.OpenIDProvider, error) {
	config := &op.Config{
		CryptoKey: key,

		// enable code_challenge_method S256 for PKCE (and therefore PKCE in general)
		CodeMethodS256: true,

		// enables additional client_id/client_secret authentication by form post (not only HTTP Basic Auth)
		AuthMethodPost: true, // TODO: do we need this?

		// enables additional authentication by using private_key_jwt
		//AuthMethodPrivateKeyJWT: true, // TODO: as alternative to client_secret?

		// enables use of the `request` Object parameter
		RequestObjectSupported: true,

		// this example has only static texts (in English), so we'll set the here accordingly
		SupportedUILocales: []language.Tag{language.English, language.Dutch},

		SupportedScopes: []string{
			oidc.ScopeOpenID,
			oidc.ScopeProfile,
			oidc.ScopeEmail,
		},
		SupportedClaims: []string{
			"sub",
			"aud",
			"exp",
			"iat",
			"iss",
			"auth_time",
			"nonce",
			"c_hash",
			"at_hash",
			"scopes",
			"client_id",
			"name",
			"email",
			"roles",
		},
	}

	opts := append([]op.Option{
		// as an example on how to customize an endpoint this will change the authorization_endpoint from /authorize to /auth
		op.WithCustomAuthEndpoint(op.NewEndpoint("auth")),
		//op.WithLogger(logger.WithGroup("openid-provider")),
	}, extraOptions...)
	handler, err := op.NewProvider(config, storage,
		func(insecure bool) (op.IssuerFromRequest, error) {
			return func(r *http.Request) string {
				return issuer
			}, nil
		}, opts...)
	if err != nil {
		return nil, err
	}
	return handler, nil
}
