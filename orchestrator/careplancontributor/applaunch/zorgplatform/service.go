package zorgplatform

import (
	"context"
	stdCrypto "crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/clients"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/crypto"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const launcherKey = "zorgplatform"
const appLaunchUrl = "/zorgplatform-app-launch"

type HcpRequester interface {
	RequestHcpRst(launchContext LaunchContext) (string, error)
}

func New(sessionManager *user.SessionManager, config Config, baseURL string, landingUrlPath string, profile profile.Provider) (*Service, error) {
	azKeysClient, err := azkeyvault.NewKeysClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	azCertClient, err := azkeyvault.NewCertificatesClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	return newWithClients(sessionManager, config, baseURL, landingUrlPath, azKeysClient, azCertClient, profile)
}

func newWithClients(sessionManager *user.SessionManager, config Config, baseURL string, landingUrlPath string,
	keysClient azkeyvault.KeysClient, certsClient azkeyvault.CertificatesClient, profile profile.Provider) (*Service, error) {
	var appLaunchURL string
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		appLaunchURL = baseURL + appLaunchUrl
	} else {
		appLaunchURL = "http://localhost" + appLaunchURL + appLaunchUrl
	}
	log.Info().Msgf("Zorgplatform app launch is: %s", appLaunchURL)

	// Load certs: signing, TLS client authentication and decryption certificates
	var signCert [][]byte
	var signCertKey crypto.Suite
	if config.AzureConfig.KeyVaultConfig.SignCertName == "" {
		if config.X509FileConfig.SignCertFile == "" {
			return nil, fmt.Errorf("no signing certificate provided in configuration")
		}
		keyPair, err := tls.LoadX509KeyPair(config.X509FileConfig.SignCertFile, config.X509FileConfig.SignKeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load signing certificate and key: %w", err)
		}
		signCert = [][]byte{keyPair.Certificate[0]}
		signCertKey = crypto.RsaSuite{PrivateKey: keyPair.PrivateKey.(*rsa.PrivateKey)}
	} else {
		var err error
		chain, key, err := azkeyvault.GetSignatureCertificate(context.Background(), certsClient, keysClient, config.AzureConfig.KeyVaultConfig.SignCertName)
		if err != nil {
			return nil, fmt.Errorf("unable to get signing certificate from Azure Key Vault: %w", err)
		}
		signCert = chain.Certificate
		signCertKey = key

	}

	var decryptCert crypto.Suite
	if config.AzureConfig.KeyVaultConfig.DecryptCertName == "" {
		if config.X509FileConfig.DecryptCertFile == "" {
			return nil, fmt.Errorf("no decryption certificate provided in configuration")
		}
		privateKeyPem, err := os.ReadFile(config.X509FileConfig.DecryptCertFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read decryption certificate from file: %w", err)
		}
		privateKeyBytes, _ := pem.Decode(privateKeyPem)
		rsaPrivateKey, err := x509.ParsePKCS8PrivateKey(privateKeyBytes.Bytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse decryption certificate: %w", err)
		}
		decryptCert = crypto.RsaSuite{PrivateKey: rsaPrivateKey.(*rsa.PrivateKey)}
	} else {
		var err error
		decryptCert, err = azkeyvault.GetKey(keysClient, config.AzureConfig.KeyVaultConfig.DecryptCertName)
		if err != nil {
			return nil, fmt.Errorf("unable to get decryption certificate from Azure Key Vault: %w", err)
		}
	}

	var tlsClientCert tls.Certificate
	if config.AzureConfig.KeyVaultConfig.ClientCertName == "" {
		if config.X509FileConfig.ClientCertFile == "" {
			return nil, fmt.Errorf("no TLS client certificate provided in configuration")
		}
		certFileContents, err := os.ReadFile(config.X509FileConfig.ClientCertFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read TLS client certificate from file: %w", err)
		}
		tlsClientCert, err = tls.X509KeyPair(certFileContents, certFileContents)
		if err != nil {
			return nil, fmt.Errorf("unable to create TLS client certificate: %w", err)
		}
	} else {
		tlsClientCertPtr, err := azkeyvault.GetTLSCertificate(context.Background(), certsClient, keysClient, config.AzureConfig.KeyVaultConfig.ClientCertName)
		if err != nil {
			return nil, fmt.Errorf("unable to get TLS client certificate from Azure Key Vault: %w", err)
		}
		tlsClientCert = *tlsClientCertPtr
	}

	result := &Service{
		sessionManager:        sessionManager,
		config:                config,
		baseURL:               baseURL,
		landingUrlPath:        landingUrlPath,
		signingCertificate:    signCert,
		signingCertificateKey: signCertKey.SigningKey(),
		tlsClientCertificate:  &tlsClientCert,
		decryptCertificate:    decryptCert,
		profile:               profile,
		// performing HTTP requests with Zorgplatform requires mutual TLS
		zorgplatformHttpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates:  []tls.Certificate{tlsClientCert},
					MinVersion:    tls.VersionTLS12,
					Renegotiation: tls.RenegotiateOnceAsClient,
				},
			},
		},
	}
	result.registerFhirClientFactory(config)
	return result, nil
}

type Service struct {
	sessionManager         *user.SessionManager
	config                 Config
	baseURL                string
	landingUrlPath         string
	signingCertificate     [][]byte
	signingCertificateKey  stdCrypto.Signer
	tlsClientCertificate   *tls.Certificate
	decryptCertificate     crypto.Suite
	zorgplatformHttpClient *http.Client
	profile                profile.Provider
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST "+appLaunchUrl, s.handleLaunch)
}

func (s *Service) handleLaunch(response http.ResponseWriter, request *http.Request) {
	log.Debug().Msg("Handling ChipSoft HiX app launch")
	if err := request.ParseForm(); err != nil {
		http.Error(response, fmt.Errorf("unable to parse form: %w", err).Error(), http.StatusBadRequest)
		return
	}
	samlResponse := request.FormValue("SAMLResponse")
	if samlResponse == "" {
		http.Error(response, "SAMLResponse not found in request", http.StatusBadRequest)
	}

	launchContext, err := s.parseSamlResponse(samlResponse)

	if err != nil {
		// Only log sensitive information, the response just sends out 400
		log.Error().Err(err).Msg("unable to validate SAML token")
		http.Error(response, "Application launch failed.", http.StatusBadRequest)
		return
	}

	// TODO: Remove this debug logging later
	log.Info().Msgf("SAML token validated, bsn=%s, workflowId=%s", launchContext.Bsn, launchContext.WorkflowId)

	//TODO: launchContext.Practitioner needs to be converted to Patient ref (after the HCP ProfessionalService access tokens can be requested)
	// Cache FHIR resources that don't exist in the EHR in the session,
	// so it doesn't collide with the EHR resources. Also prefix it with a magic string to make it clear it's special.

	// Use the launch context to retrieve an access_token that allows the application to query the HCP ProfessionalService
	accessToken, err := s.RequestHcpRst(launchContext)

	if err != nil {
		log.Error().Err(err).Msg("unable to request access token for HCP ProfessionalService")
		http.Error(response, "Application launch failed.", http.StatusBadRequest)
		return
	}

	log.Info().Msgf("Successfully requested access token for HCP ProfessionalService, access_token=%s...", accessToken[:16])

	sessionData, err := s.getSessionData(request.Context(), accessToken, launchContext)
	if err != nil {
		log.Error().Err(err).Msg("unable to create session data")
		http.Error(response, "Application launch failed.", http.StatusInternalServerError)
		return
	}

	s.sessionManager.Create(response, *sessionData)

	// Redirect to landing page
	targetURL, _ := url.Parse(s.baseURL)
	targetURL = targetURL.JoinPath(s.landingUrlPath)
	log.Info().Msg("Successfully launched through ChipSoft HiX app launch")
	http.Redirect(response, request, targetURL.String(), http.StatusFound)
}

func (s *Service) registerFhirClientFactory(config Config) {
	// Register FHIR client factory that can create FHIR clients when the Zorgplatform AppLaunch is used
	clients.Factories[launcherKey] = func(properties map[string]string) clients.ClientProperties {
		fhirServerURL, _ := url.Parse(config.ApiUrl)
		return clients.ClientProperties{
			BaseURL: fhirServerURL,
			Client:  s.createZorgplatformApiClient(properties["accessToken"]).Transport,
		}
	}
}

func (s *Service) createZorgplatformApiClient(accessToken string) *http.Client {
	return &http.Client{
		Transport: &authHeaderRoundTripper{
			value: "SAML " + accessToken,
			inner: s.zorgplatformHttpClient.Transport,
		},
	}
}

func (s *Service) getSessionData(ctx context.Context, accessToken string, launchContext LaunchContext) (*user.SessionData, error) {
	// New client that uses the access token
	apiUrl, err := url.Parse(s.config.ApiUrl)
	if err != nil {
		return &user.SessionData{}, err
	}
	fhirClient := fhirclient.New(apiUrl, s.createZorgplatformApiClient(accessToken), coolfhir.Config())
	// Zorgplatform provides us with a Task, from which we need to derive the Patient and ServiceRequest
	// - Patient is contained in the Task.encounter but separately requested to include the Practitioner
	// - ServiceRequest is not provided by Zorgplatform, so we need to create one based on the Task and Encounter
	var task map[string]interface{}
	if err = fhirClient.ReadWithContext(ctx, "Task/"+launchContext.WorkflowId, &task); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to fetch Task resource (id=%s): %w", launchContext.WorkflowId, err)
	}
	// Search for Encounter, contains Encounter, Organization and Patient
	if task["context"] == nil {
		return &user.SessionData{}, fmt.Errorf("task.context is not provided")
	}
	taskContext := task["context"].(map[string]interface{})
	encounterId := strings.Split(taskContext["reference"].(string), "/")[1]
	var encounterSearchResult fhir.Bundle
	if err = fhirClient.ReadWithContext(ctx, "Encounter", &encounterSearchResult, fhirclient.QueryParam("_id", encounterId)); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to fetch Encounter resource (id=%s): %w", encounterId, err)
	}
	var encounter fhir.Encounter
	if err := coolfhir.ResourceInBundle(&encounterSearchResult, coolfhir.EntryIsOfType("Encounter"), &encounter); err != nil {
		return &user.SessionData{}, fmt.Errorf("get Encounter from Bundle (id=%s): %w", encounterId, err)
	}
	// Get Patient from bundle, specified by Encounter.subject
	if encounter.Subject == nil || encounter.Subject.Reference == nil {
		return &user.SessionData{}, fmt.Errorf("encounter.subject does not contain a reference to a Patient")
	}

	if encounter.ServiceProvider == nil || encounter.ServiceProvider.Reference == nil {
		return &user.SessionData{}, fmt.Errorf("encounter.serviceProvider does not contain a reference to an Organization")
	}

	// Get Organization from bundle, specified by Encounter.serviceProvider
	var organization fhir.Organization
	if err := coolfhir.ResourceInBundle(&encounterSearchResult, coolfhir.EntryHasID(*encounter.ServiceProvider.Reference), &organization); err != nil {
		return &user.SessionData{}, fmt.Errorf("get Organization from Bundle (id=%s): %w", encounterId, err)
	}

	var patientAndPractitionerBundle fhir.Bundle
	if err = fhirClient.ReadWithContext(ctx, "Patient", &patientAndPractitionerBundle, fhirclient.QueryParam("_include", "Patient:general-practitioner")); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to fetch Patient and Practitioner bundle: %w", err)
	}
	var patient fhir.Patient
	if err := coolfhir.ResourceInBundle(&patientAndPractitionerBundle, coolfhir.EntryHasID(*encounter.Subject.Reference), &patient); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to find Patient resource in Bundle (id=%s): %w", *encounter.Subject.Reference, err)
	}
	var practitioner fhir.Practitioner
	//TODO: The Practitioner has no indetifier set, so we cannot ensure this is the launched Practitioner. Verify with Zorgplatform
	// if err := coolfhir.ResourceInBundle(&patientAndPractitionerBundle, coolfhir.EntryHasIdentifier(launchContext.Practitioner.Identifier[0]), &practitioner); err != nil {
	if err := coolfhir.ResourceInBundle(&patientAndPractitionerBundle, coolfhir.EntryIsOfType("Practitioner"), &practitioner); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to find Practitioner resource in Bundle: %w", err)
	}

	var conditionBundle fhir.Bundle
	var conditions []map[string]interface{}
	//TODO: We assume this is in context as the HCP token is workflow-specific. Double-check with Zorgplatform
	if err = fhirClient.ReadWithContext(ctx, "Condition", &conditionBundle, fhirclient.QueryParam("subject", "Patient/"+*patient.Id)); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to fetch Conditions bundle: %w", err)
	}
	if err := coolfhir.ResourcesInBundle(&conditionBundle, coolfhir.EntryIsOfType("Condition"), &conditions); err != nil {
		return &user.SessionData{}, fmt.Errorf("unable to find Condition resources in Bundle: %w", err)
	}

	var reasonReferences []fhir.Reference
	var conditionMap = make(map[string]interface{})

	for _, condition := range conditions {
		ref := "Condition/magic-" + condition["id"].(string)
		reasonReferences = append(reasonReferences, fhir.Reference{
			Type:      to.Ptr("Condition"),
			Reference: to.Ptr(ref),
		})
		conditionMap[ref] = condition
	}

	identities, err := s.profile.Identities(ctx)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch identities: %w", err)
	}

	uraIdentifierReferences := make([]fhir.Reference, 0, len(identities))
	for _, identity := range identities {
		uraIdentifierReferences = append(uraIdentifierReferences, fhir.Reference{
			Identifier: &identity,
			Display:    to.Ptr("Zorgbijjou"), //TODO: Remove hard coded value
		})
	}

	var conditionText string
	if len(conditions) > 0 {
		if code, ok := conditions[0]["code"].(map[string]interface{}); ok {
			if text, ok := code["text"].(string); ok {
				conditionText = text
			} else {
				log.Warn().Msg("condition code does not contain text - using default hard-coded value")
			}
		} else {
			log.Warn().Msg("condition does not contain code - using default hard-coded value")
		}
	} else {
		return nil, fmt.Errorf("no conditions found")
	}
	if conditionText == "" {
		conditionText = "fractuur van pols" //TODO: Hard coded value for demo purposes"
	}

	// Zorgplatform does not provide a ServiceRequest, so we need to create one based on other resources they do use
	serviceRequest := &fhir.ServiceRequest{
		Status: fhir.RequestStatusActive,
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					Display: to.Ptr(conditionText),
				},
			},
		},
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("tmp"), //TODO: Hard coded to fractuur-pols for demo purposes. Should be based on the Task
				Value:  to.Ptr("fractuur-pols"),
			},
		},
		ReasonReference: reasonReferences,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  &launchContext.Bsn,
			},
		},
		Requester: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  &s.config.TaskPerformerUra,
			},
		},
		Performer: uraIdentifierReferences,
	}

	patientRef := "Patient/magic-" + uuid.NewString()
	serviceRequestRef := "ServiceRequest/magic-" + uuid.NewString()
	practitionerRef := "Practitioner/magic-" + uuid.NewString()
	organizationRef := "Organization/magic-" + uuid.NewString()

	otherValues := map[string]interface{}{
		patientRef:        patient,
		practitionerRef:   practitioner,
		serviceRequestRef: *serviceRequest,
		organizationRef:   organization,
		"Condition":       conditions,
		"launchContext":   launchContext, // Can be used to fetch a new access token after expiration
	}

	//inject the conditions into the "other" values
	for k, v := range conditionMap {
		otherValues[k] = v
	}

	return &user.SessionData{
		FHIRLauncher: launcherKey,
		//TODO: See how/if to pass the conditions to the StringValues
		StringValues: map[string]string{
			"patient":        patientRef,
			"serviceRequest": serviceRequestRef,
			"practitioner":   practitionerRef,
			"organization":   organizationRef,
			"accessToken":    accessToken,
		},
		OtherValues: otherValues,
	}, nil
}

type authHeaderRoundTripper struct {
	value string
	inner http.RoundTripper
}

func (a authHeaderRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Add("Authorization", a.value)
	return a.inner.RoundTrip(request)
}
