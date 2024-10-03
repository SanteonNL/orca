package zorgplatform

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
)

const fhirLauncherKey = "zorgplatform"

func New(sessionManager *user.SessionManager, config Config, baseURL string, landingUrlPath string) *Service {
	var appLaunchURL string
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		appLaunchURL = baseURL + "/zorgplatform-launch"
	} else {
		appLaunchURL = "http://localhost" + appLaunchURL + "/zorgplatform-launch"
	}
	log.Info().Msgf("Zorgplatform app launch is (%s)", appLaunchURL)

	// Register FHIR client factory that can create FHIR clients when the Zorgplatform AppLaunch is used
	clients.Factories[fhirLauncherKey] = func(properties map[string]string) clients.ClientProperties {
		fhirServerURL, _ := url.Parse(config.ApiUrl)
		return clients.ClientProperties{
			BaseURL: fhirServerURL,
			Client:  http.DefaultTransport,
		}
	}

	return &Service{
		sessionManager: sessionManager,
		config:         config,
		baseURL:        baseURL,
		landingUrlPath: landingUrlPath,
	}
}

type Service struct {
	sessionManager *user.SessionManager
	config         Config
	baseURL        string
	landingUrlPath string
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST /zorgplatform-launch", s.handle)
}

func (s *Service) handle(response http.ResponseWriter, request *http.Request) {

	encryptedToken, err := s.getEncryptedSAMLToken(response, request)

	if err != nil {
		log.Error().Err(err).Msg("unable to get SAML token")
		http.Error(response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	launchContext, err := s.validateEncryptedSAMLToken(encryptedToken)

	if err != nil {
		//Only log sensitive information, the response just sends out 400
		log.Error().Err(err).Msg("unable to validate SAML token")
		http.Error(response, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	//TODO: launchContext.Subject needs to be converted to Patient ref (after the HCP ProfessionalService access tokens can be requested)
	s.sessionManager.Create(response, user.SessionData{
		FHIRLauncher: fhirLauncherKey,
		Values: map[string]string{
			// "context":        launchContext,
			"subject": launchContext.Subject,
			// "patient":        launchContext.Patient,
			// "practitioner":   launchContext.Practitioner,
			// "serviceRequest": launchContext.ServiceRequest,
			// "iss":            launchContext.Issuer,
		},
	})

	// Redirect to landing page
	targetURL, _ := url.Parse(s.baseURL)
	targetURL = targetURL.JoinPath(s.landingUrlPath)

	http.Redirect(response, request, targetURL.String(), http.StatusFound)
}

func (s *Service) getEncryptedSAMLToken(response http.ResponseWriter, request *http.Request) (token string, err error) {
	body, err := io.ReadAll(request.Body)

	if err != nil {
		return "", fmt.Errorf("unable to read request body: %w", err)
	}
	defer request.Body.Close()

	// Extract SAMLResponse from the body
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return "", fmt.Errorf("unable to parse request body: %w", err)
	}
	samlResponse := values.Get("SAMLResponse")
	if samlResponse == "" {
		return "", fmt.Errorf("SAMLResponse not found in request body")
	}

	return samlResponse, nil
}
