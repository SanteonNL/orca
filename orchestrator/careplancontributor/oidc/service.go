package oidc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zitadel/oidc/v3/pkg/op"
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
	key := [32]byte{} // TODO: what is this used for?

	// TODO: Change to key in Azure Key Vault
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}
	keyID := uuid.NewString()
	signingKey := SigningKey{
		id:           keyID,
		sigAlgorithm: "ES256",
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
	// TODO: Get these from the authenticated session
	err := s.storage.AuthenticateUser(httpRequest.Context(), authRequestID, UserDetails{
		ID:    "12345",
		Name:  "John Doe",
		Email: "john@example.com",
		Role:  "Verpleegkundige niveau 4",
	})
	if err != nil {
		http.Error(httpResponse, fmt.Errorf("authenticate user: %w", err).Error(), http.StatusInternalServerError)
		return
	}
	redirectURL := s.callbackURLFunc(httpRequest.Context(), authRequestID)
	redirectURL = s.issuerURL.JoinPath(redirectURL).Path
	http.Redirect(httpResponse, httpRequest, redirectURL, http.StatusFound)
}

func (s *Service) ServeHTTP(httpResponse http.ResponseWriter, httpRequest *http.Request) {
	s.provider.ServeHTTP(httpResponse, httpRequest)
}

func newOIDCProvider(storage op.Storage, issuer string, key [32]byte, logger *slog.Logger, extraOptions []op.Option) (op.OpenIDProvider, error) {
	config := &op.Config{
		CryptoKey: key,

		// will be used if the end_session endpoint is called without a post_logout_redirect_uri
		//DefaultLogoutRedirectURI: pathLoggedOut, // TODO

		// enable code_challenge_method S256 for PKCE (and therefore PKCE in general)
		CodeMethodS256: true,

		// enables additional client_id/client_secret authentication by form post (not only HTTP Basic Auth)
		AuthMethodPost: true, // TODO: do we need this?

		// enables additional authentication by using private_key_jwt
		//AuthMethodPrivateKeyJWT: true, // TODO: as alternative to client_secret?

		// enables refresh_token grant use
		// GrantTypeRefreshToken: true, // TODO: not needed?

		// enables use of the `request` Object parameter
		RequestObjectSupported: true,

		// this example has only static texts (in English), so we'll set the here accordingly
		SupportedUILocales: []language.Tag{language.English, language.Dutch},
	}
	opts := append([]op.Option{
		// as an example on how to customize an endpoint this will change the authorization_endpoint from /authorize to /auth
		op.WithCustomAuthEndpoint(op.NewEndpoint("auth")),
		op.WithLogger(logger.WithGroup("openid-provider")),
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
