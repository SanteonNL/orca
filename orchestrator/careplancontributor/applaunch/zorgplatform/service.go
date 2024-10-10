package zorgplatform

import (
	"context"
	"crypto/x509"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/crypto"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"strings"
)

const fhirLauncherKey = "zorgplatform"
const appLaunchUrl = "/zorgplatform-app-launch"

func New(sessionManager *user.SessionManager, config Config, baseURL string, landingUrlPath string) (*Service, error) {
	var appLaunchURL string
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		appLaunchURL = baseURL + appLaunchUrl
	} else {
		appLaunchURL = "http://localhost" + appLaunchURL + appLaunchUrl
	}
	log.Info().Msgf("Zorgplatform app launch is (%s)", appLaunchURL)

	registerFhirClientFactory(config)

	// Load certs: signing, TLS client authentication and decryption certificates
	azKeysClient, err := azkeyvault.NewKeysClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	azCertClient, err := azkeyvault.NewCertificatesClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	signingCert, err := azkeyvault.GetKey(azKeysClient, config.AzureConfig.KeyVaultConfig.SignCertName, config.AzureConfig.KeyVaultConfig.SignCertVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get signing certificate from Azure Key Vault: %w", err)
	}
	decryptCert, err := azkeyvault.GetKey(azKeysClient, config.AzureConfig.KeyVaultConfig.DecryptCertName, config.AzureConfig.KeyVaultConfig.DecryptCertVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get decryption certificate from Azure Key Vault: %w", err)
	}
	tlsClientCert, err := azkeyvault.GetCertificate(context.Background(), azCertClient, config.AzureConfig.KeyVaultConfig.ClientCertName, config.AzureConfig.KeyVaultConfig.ClientCertVersion)
	if err != nil {
		return nil, fmt.Errorf("unable to get TLS client certificate from Azure Key Vault: %w", err)
	}
	return &Service{
		sessionManager:       sessionManager,
		config:               config,
		baseURL:              baseURL,
		landingUrlPath:       landingUrlPath,
		signingCertificate:   signingCert,
		tlsClientCertificate: tlsClientCert,
		decryptCertificate:   decryptCert,
	}, nil
}

type Service struct {
	sessionManager       *user.SessionManager
	config               Config
	baseURL              string
	landingUrlPath       string
	signingCertificate   crypto.Suite
	tlsClientCertificate *x509.Certificate
	decryptCertificate   crypto.Suite
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST "+appLaunchUrl, s.handle)
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

	assertionString, _ := launchContext.DecryptedAssertion.WriteToString()
	log.Info().Msgf("SAML token validated, subject=%s, bsn=%s, workflowId=%s, decryptedAssertion=%s", launchContext.Subject, launchContext.Bsn, launchContext.WorkflowId, assertionString)

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
	if err = request.ParseForm(); err != nil {
		return "", fmt.Errorf("unable to parse form: %w", err)
	}
	value := request.FormValue("SAMLResponse")
	if value == "" {
		return "", fmt.Errorf("SAMLResponse not found in request")
	}
	return value, nil
}

func registerFhirClientFactory(config Config) {
	// Register FHIR client factory that can create FHIR clients when the Zorgplatform AppLaunch is used
	clients.Factories[fhirLauncherKey] = func(properties map[string]string) clients.ClientProperties {
		fhirServerURL, _ := url.Parse(config.ApiUrl)
		return clients.ClientProperties{
			BaseURL: fhirServerURL,
			Client:  http.DefaultTransport,
		}
	}
}
