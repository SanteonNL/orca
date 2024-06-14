package smartonfhir

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Service struct {
	config             Config
	stateToTokenUrlMap map[string]string //TODO: move to redis
	mu                 *sync.Mutex
}

func New(config Config) *Service {
	return &Service{
		config:             config,
		stateToTokenUrlMap: make(map[string]string),
		mu:                 &sync.Mutex{},
	}
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("GET /smart-app-launch", s.handleSmartAppLaunch)
	mux.HandleFunc("GET /smart-app-launch/redirect", s.handleSmartAppLaunchRedirect)
}

func (s *Service) handleSmartAppLaunch(response http.ResponseWriter, request *http.Request) {
	iss, err := url.Parse(request.URL.Query().Get("iss"))
	if err != nil {
		http.Error(response, "invalid iss parameter", http.StatusBadRequest)
		return
	}
	launch := request.URL.Query().Get("launch")

	authURL, err := s.appLaunchServiceLogic(iss, launch)

	if err != nil {
		http.Error(response, err.Error(), http.StatusInternalServerError)
		return
	}

	//TODO: Add client_secret Auth header?
	http.Redirect(response, request, authURL, http.StatusFound)
}

func (s *Service) appLaunchServiceLogic(issuer *url.URL, launch string) (string, error) {
	config, err := DiscoverConfiguration(issuer)
	if err != nil {
		return "", err
	}
	if config.AuthorizationEndpoint == "" {
		return "", errors.New("authorization endpoint not found in OpenID configuration")
	}
	if config.TokenEndpoint == "" {
		return "", errors.New("token endpoint not found in OpenID configuration")
	}

	// Generate a UUID v4 for the state parameter & map the token endpoint to the state value for after redirect
	state := uuid.New().String()

	s.mu.Lock()
	s.stateToTokenUrlMap[state] = config.TokenEndpoint
	s.mu.Unlock()

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", s.config.ClientID)
	query.Set("redirect_uri", s.config.RedirectURI)
	query.Set("scope", s.config.Scope)
	query.Set("launch", launch)
	query.Set("aud", issuer.String())
	query.Set("state", state)

	authURL := config.AuthorizationEndpoint + "?" + query.Encode()
	return authURL, nil
}

func (s *Service) handleSmartAppLaunchRedirect(response http.ResponseWriter, request *http.Request) {

	state := request.URL.Query().Get("state")
	code := request.URL.Query().Get("code")

	tokenResponse, err := s.appLaunchRedirectLogic(state, code)
	if err != nil {
		return
	}

	// accessToken := tokenResponse["access_token"]

	log.Info().Msgf("SMART App Launch succeeded, got the following response\n%v", tokenResponse)

	_, _ = response.Write([]byte("Launch succeeded"))

	// 1) Extract the type of launch that is being performed, for example an enrollment, or a data view
	// 2) switch type - call the apropriate service to handle the request
}

func (s *Service) appLaunchRedirectLogic(state string, code string) (map[string]interface{}, error) {
	s.mu.Lock()
	tokenEndpoint, ok := s.stateToTokenUrlMap[state]
	s.mu.Unlock()

	if !ok {
		return nil, errors.New("invalid state parameter - token endpoint not found")
	}

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", s.config.RedirectURI)
	data.Set("client_id", s.config.ClientID)

	resp, err := http.PostForm(tokenEndpoint, data)
	if err != nil {
		log.Error().Err(err).Msg("Failed to exchange authorization code for access token")
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to exchange authorization code for access token")
	}

	var tokenResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		log.Error().Err(err).Msg("Failed to decode token response")
		return nil, err
	}

	s.mu.Lock()
	delete(s.stateToTokenUrlMap, state)
	s.mu.Unlock()
	return tokenResponse, nil
}
