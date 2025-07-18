package zorgplatform

import (
	"context"
	stdCrypto "crypto"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/SanteonNL/orca/orchestrator/globals"

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
	"github.com/jellydator/ttlcache/v3"
	"github.com/rs/zerolog/log"
)

const launcherKey = "zorgplatform"
const appLaunchUrl = "/zorgplatform-app-launch"
const zorgplatformWorkflowIdSystem = "http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"

// accessTokenCacheTTL is the time-to-live for the Zorgplatform access tokens in the cache.
// They expire after ca. 12 minutes, so this value is on the safe side.
const accessTokenCacheTTL = time.Minute * 5

var sleep = time.Sleep

type workflowContext struct {
	workflowId string
	patientBsn string
}

type OtherSessionData struct {
	LaunchContext LaunchContext
	AccessToken   string
}

func New(sessionManager *user.SessionManager[session.Data], config Config, baseURL string, frontendLandingUrl *url.URL, profile profile.Provider) (*Service, error) {
	azKeysClient, err := azkeyvault.NewKeysClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}
	azCertClient, err := azkeyvault.NewCertificatesClient(config.AzureConfig.KeyVaultConfig.KeyVaultURL, config.AzureConfig.CredentialType, false)
	if err != nil {
		return nil, fmt.Errorf("unable to create Azure Key Vault client: %w", err)
	}

	ctx := context.Background()

	return newWithClients(ctx, sessionManager, config, baseURL, frontendLandingUrl, azKeysClient, azCertClient, profile)
}

func newWithClients(ctx context.Context, sessionManager *user.SessionManager[session.Data], config Config, baseURL string, frontendLandingUrl *url.URL,
	keysClient azkeyvault.KeysClient, certsClient azkeyvault.CertificatesClient, profile profile.Provider) (*Service, error) {
	var appLaunchURL string
	if strings.HasPrefix(baseURL, "http://") || strings.HasPrefix(baseURL, "https://") {
		appLaunchURL = baseURL + appLaunchUrl
	} else {
		appLaunchURL = "http://localhost" + appLaunchURL + appLaunchUrl
	}
	log.Ctx(ctx).Info().Msgf("Zorgplatform app launch is: %s", appLaunchURL)

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

	zorgplatformSignCerts, err := getCertificates(ctx, config.DecryptConfig.SignCertPem)
	if err != nil {
		return nil, fmt.Errorf("unable to load Zorgplatform's public signing certificate: %w", err)
	}

	result := &Service{
		sessionManager:        sessionManager,
		config:                config,
		baseURL:               baseURL,
		frontendLandingUrl:    frontendLandingUrl,
		signingCertificate:    signCert,
		signingCertificateKey: signCertKey.SigningKey(),
		tlsClientCertificate:  &tlsClientCert,
		decryptCertificate:    decryptCert,
		zorgplatformSignCerts: zorgplatformSignCerts,
		profile:               profile,
		accessTokenCache: ttlcache.New[string, string](
			ttlcache.WithTTL[string, string](accessTokenCacheTTL),
		),
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
	// Start cache expiry
	go result.accessTokenCache.Start()

	result.secureTokenService = result
	result.registerFhirClientFactory(config)
	result.getSessionData = result.defaultGetSessionData

	return result, nil
}

type Service struct {
	sessionManager         *user.SessionManager[session.Data]
	config                 Config
	baseURL                string
	frontendLandingUrl     *url.URL
	signingCertificate     [][]byte
	signingCertificateKey  stdCrypto.Signer
	tlsClientCertificate   *tls.Certificate
	decryptCertificate     crypto.Suite
	zorgplatformHttpClient *http.Client
	zorgplatformSignCerts  []*x509.Certificate
	profile                profile.Provider
	secureTokenService     SecureTokenService
	getSessionData         func(ctx context.Context, accessToken string, launchContext LaunchContext) (*session.Data, error)
	// accessTokenCache stores the Zorgplatform access tokens a given CarePlan (from X-SCP-Context)
	// Requesting an access token involves a chained lookup, so we cache the result for some time.
	accessTokenCache *ttlcache.Cache[string, string]
}

func (s *Service) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("POST "+appLaunchUrl, s.handleLaunch)
}

func (s *Service) EhrFhirProxy() coolfhir.HttpProxy {
	targetFhirBaseUrl, _ := url.Parse(s.config.ApiUrl)
	const proxyBasePath = "/cpc/fhir"
	rewriteUrl, _ := url.Parse(s.baseURL)
	rewriteUrl = rewriteUrl.JoinPath(proxyBasePath)
	result := coolfhir.NewProxy("App->EHR (ZPF)", targetFhirBaseUrl, proxyBasePath, rewriteUrl, &stsAccessTokenRoundTripper{
		transport:          s.zorgplatformHttpClient.Transport,
		cpsFhirClient:      s.cpsFhirClient,
		secureTokenService: s.secureTokenService,
		accessTokenCache:   s.accessTokenCache,
	}, true, false)
	// Zorgplatform's FHIR API only allows GET-based FHIR searches, while ORCA only allows POST-based FHIR searches.
	// If the request is a POST-based search, we need to rewrite the request to a GET-based search.
	result.HTTPRequestModifier = func(req *http.Request) (*http.Request, error) {
		if strings.HasSuffix(req.URL.Path, "_search") && req.Method == http.MethodPost {
			newReq := req.Clone(req.Context())
			newReq.Method = http.MethodGet
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			newReq.URL.RawQuery = req.Form.Encode()
			newReq.URL.Path = strings.TrimSuffix(req.URL.Path, "/_search")
			newReq.Body = nil
			return newReq, nil
		}
		return req, nil
	}
	return result
}

var _ http.RoundTripper = &stsAccessTokenRoundTripper{}

type stsAccessTokenRoundTripper struct {
	transport          http.RoundTripper
	cpsFhirClient      func() fhirclient.Client
	secureTokenService SecureTokenService
	accessTokenCache   *ttlcache.Cache[string, string]
}

func (s *stsAccessTokenRoundTripper) RoundTrip(httpRequest *http.Request) (*http.Response, error) {
	log.Ctx(httpRequest.Context()).Debug().Msg("Handling Zorgplatform STS access token request")

	// Do something to request the access token
	newHttpRequest := httpRequest.Clone(httpRequest.Context())

	//TODO: Check if we can update the CarePlan.basedOn to point to the service request
	carePlanReference := httpRequest.Header.Get("X-Scp-Context")
	if carePlanReference == "" {
		log.Ctx(httpRequest.Context()).Error().Msg("Missing X-Scp-Context header")
		return nil, fmt.Errorf("missing X-Scp-Context header")
	}

	log.Ctx(httpRequest.Context()).Debug().Msgf("Found SCP context: %s", carePlanReference)
	// First see if cached
	var accessToken string
	if cacheEntry := s.accessTokenCache.Get(carePlanReference); cacheEntry != nil {
		accessToken = cacheEntry.Value()
	} else {
		log.Ctx(httpRequest.Context()).Debug().Msgf("(cache miss) Getting Zorgplatform access token for CarePlan reference: %s", carePlanReference)
		workflowCtx, err := s.getWorkflowContext(httpRequest.Context(), carePlanReference)
		if err != nil {
			log.Ctx(httpRequest.Context()).Error().Msgf("Unable to get workflowId for CarePlan reference: %v", err)
			return nil, fmt.Errorf("unable to get workflowId for CarePlan reference: %w", err)
		}

		//TODO: Below is to solve a bug in zorgplatform. The SAML attribute contains bsn "999911120", but the actual patient has bsn "999999151" in the resource/workflow context
		if !globals.StrictMode {
			log.Ctx(httpRequest.Context()).Warn().Msg("Applying workaround for Zorgplatform BSN testdata bug (changing BSN 999911120 to 999999151)")
			if workflowCtx.patientBsn == "999911120" {
				workflowCtx.patientBsn = "999999151"
			}
		}

		launchContext := LaunchContext{
			WorkflowId: workflowCtx.workflowId,
			Bsn:        workflowCtx.patientBsn,
		}

		accessToken, err = s.secureTokenService.RequestAccessToken(httpRequest.Context(), launchContext, applicationTokenType)
		if err != nil {
			log.Ctx(httpRequest.Context()).Error().Msgf("Unable to request access token for Zorgplatform: %v", err)
			return nil, fmt.Errorf("unable to request access token for Zorgplatform: %w", err)
		}
		log.Ctx(httpRequest.Context()).Debug().Msgf("Successfully requested access token for Zorgplatform, access_token=%s...", accessToken[:min(len(accessToken), 16)])
		s.accessTokenCache.Set(carePlanReference, accessToken, ttlcache.DefaultTTL)
	}

	newHttpRequest.Header.Add("Accept", "application/fhir+json")
	newHttpRequest.Header.Add("Authorization", "SAML "+accessToken)
	return s.transport.RoundTrip(newHttpRequest)
}

func (s *Service) handleLaunch(response http.ResponseWriter, request *http.Request) {
	log.Ctx(request.Context()).Debug().Msg("Handling ChipSoft HiX app launch")
	if err := request.ParseForm(); err != nil {
		http.Error(response, fmt.Errorf("unable to parse form: %w", err).Error(), http.StatusBadRequest)
		return
	}
	samlResponse := request.FormValue("SAMLResponse")
	if samlResponse == "" {
		http.Error(response, "SAMLResponse not found in request", http.StatusBadRequest)
		return
	}

	launchContext, err := s.parseSamlResponse(request.Context(), samlResponse)

	if err != nil {
		// Only log sensitive information, the response just sends out 400
		log.Ctx(request.Context()).Err(err).Msg("unable to validate SAML token")
		http.Error(response, "Application launch failed.", http.StatusBadRequest)
		return
	}

	// TODO: Remove this debug logging later
	log.Ctx(request.Context()).Info().Msgf("SAML token validated, bsn=%s, workflowId=%s", launchContext.Bsn, launchContext.WorkflowId)

	//TODO: launchContext.Practitioner needs to be converted to Patient ref (after the HCP ProfessionalService access tokens can be requested)
	// Cache FHIR resources that don't exist in the EHR in the session,
	// so it doesn't collide with the EHR resources. Also prefix it with a magic string to make it clear it's special.

	// Use the launch context to retrieve an access_token that allows the application to query the HCP ProfessionalService
	accessToken, err := s.secureTokenService.RequestAccessToken(request.Context(), launchContext, hcpTokenType)
	if err != nil {
		log.Ctx(request.Context()).Err(err).Msg("unable to request access token for HCP ProfessionalService")
		http.Error(response, "Application launch failed.", http.StatusBadRequest)
		return
	}

	log.Ctx(request.Context()).Info().Msgf("Successfully requested access token for HCP ProfessionalService, access_token=%s...", accessToken[:min(len(accessToken), 16)])

	var sessionData *session.Data
	// INT-572: Adding a retry mechanism, as in some cases the Patient returns a 404 when a new workflow is created and directly launched in HiX
	for i := 1; i <= 4; i++ {
		sessionData, err = s.getSessionData(request.Context(), accessToken, launchContext)
		if err == nil {
			break
		}
		if i < 4 {
			log.Ctx(request.Context()).Warn().Msg("unable to create session data, retrying")
			sleep(200 * time.Duration(i) * time.Millisecond)
		} else {
			log.Ctx(request.Context()).Err(err).Msg("unable to create session data - retry limit reached")
			http.Error(response, "Application launch failed.", http.StatusInternalServerError)
			return
		}
	}

	s.sessionManager.Create(response, *sessionData)

	// Redirect to landing page
	log.Ctx(request.Context()).Info().Msg("Successfully launched through ChipSoft HiX app launch")

	taskResource := sessionData.GetByType("Task")
	redirectURL := s.frontendLandingUrl
	// If the task doesn't exist yet, send the user to the new page, where data is first confirmed
	if taskResource == nil {
		redirectURL = redirectURL.JoinPath("new")
	} else {
		// taskRef is in format Task/<id>, redirect URL is in task/<id> format
		redirectURL = redirectURL.JoinPath("task", strings.Split(taskResource.Path, "/")[1])
	}
	http.Redirect(response, request, redirectURL.String(), http.StatusFound)
}

// This function returns the workflowId for a given CarePlan reference. It will be returned from a cache, if available.
func (s *stsAccessTokenRoundTripper) getWorkflowContext(ctx context.Context, carePlanReference string) (*workflowContext, error) {
	// TODO: use baseURL if there's multiple possible CPS'
	_, localCarePlanRef, err := coolfhir.ParseExternalLiteralReference(carePlanReference, "CarePlan")
	if err != nil {
		return nil, fmt.Errorf("invalid CarePlan reference (url=%s): %w", carePlanReference, err)
	}
	log.Ctx(ctx).Debug().Msgf("Fetching CarePlan resource: %s", localCarePlanRef)

	// TODO: group requests, something like:
	// ?_include=CarePlan:activity-reference&_include:iterate=Task:focus
	// query.Add("_include", "CarePlan:activity-reference")
	// //TODO: This is a tmp solution, but if it's not that temporary, not all FHIR servers support chaining
	// query.Add("_include:iterate", "Task:focus") //chain the Task.focus from the joined CarePlan.activity-references
	// urlRef.RawQuery = query.Encode()

	var carePlan fhir.CarePlan
	err = s.cpsFhirClient().ReadWithContext(ctx, localCarePlanRef, &carePlan)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Unable to fetch CarePlan resource: %v", err)
		return nil, fmt.Errorf("unable to fetch CarePlan resource: %w", err)
	}

	// TODO: What if there's no activities?
	var task fhir.Task
	err = s.cpsFhirClient().ReadWithContext(ctx, *carePlan.Activity[0].Reference.Reference, &task)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Unable to fetch Task resource: %v", err)
		return nil, fmt.Errorf("unable to fetch Task resource: %w", err)
	}

	var serviceRequest fhir.ServiceRequest
	err = s.cpsFhirClient().ReadWithContext(ctx, *task.Focus.Reference, &serviceRequest)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Unable to fetch ServiceRequest resource: %v", err)
		return nil, fmt.Errorf("unable to fetch ServiceRequest resource: %w", err)
	}

	workflowIdIdentifiers := coolfhir.FilterIdentifier(&serviceRequest.Identifier, zorgplatformWorkflowIdSystem)
	if len(workflowIdIdentifiers) != 1 {
		return nil, fmt.Errorf("expected ServiceRequest to have 1 identifier with system %s", zorgplatformWorkflowIdSystem)
	}
	workflowID := *workflowIdIdentifiers[0].Value

	var patient fhir.Patient
	err = s.cpsFhirClient().ReadWithContext(ctx, *carePlan.Subject.Reference, &patient)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Unable to fetch Patient resource: %v", err)
		return nil, fmt.Errorf("unable to fetch Patient resource: %w", err)
	}

	bsnIdentifier := coolfhir.FilterFirstIdentifier(&patient.Identifier, coolfhir.BSNNamingSystem)
	if bsnIdentifier == nil {
		return nil, fmt.Errorf("identifier with system %s not found", coolfhir.BSNNamingSystem)
	}
	result := &workflowContext{
		workflowId: workflowID,
		patientBsn: *bsnIdentifier.Value,
	}
	return result, nil
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

func (s *Service) defaultGetSessionData(ctx context.Context, accessToken string, launchContext LaunchContext) (*session.Data, error) {
	// New client that uses the access token
	apiUrl, err := url.Parse(s.config.ApiUrl)
	if err != nil {
		return nil, err
	}

	var existingTaskRef *string
	existingTask, err := coolfhir.GetTaskByIdentifier(ctx, s.cpsFhirClient(), fhir.Identifier{
		System: to.Ptr(zorgplatformWorkflowIdSystem),
		Value:  to.Ptr(launchContext.WorkflowId),
	})

	if err != nil {
		log.Ctx(ctx).Error().Msgf("Failed to check for existing CPS Task resource: %v", err)
		return nil, fmt.Errorf("failed to check for existing CPS Task resource: %w", err)
	}

	if existingTask != nil {
		existingTaskRef = to.Ptr("Task/" + *existingTask.Id)
	}

	fhirClient := fhirclient.New(apiUrl, s.createZorgplatformApiClient(accessToken), coolfhir.Config())

	// Zorgplatform provides us with a Task, from which we need to derive the Patient and ServiceRequest
	// - ServiceRequest is not provided by Zorgplatform, so we need to create one based on the Task
	var task map[string]interface{}
	if err = fhirClient.ReadWithContext(ctx, "Task/"+launchContext.WorkflowId, &task); err != nil {
		return nil, fmt.Errorf("unable to fetch Task resource (id=%s): %w", launchContext.WorkflowId, err)
	}
	// Determine service and condition code from the workflow specified by Task.definitionReference.reference (e.g. Telemonitoring, COPD)
	conditionCode, err := getConditionCodeFromWorkflowTask(task)
	if err != nil {
		return nil, err
	}

	var patientAndPractitionerBundle fhir.Bundle
	if err = fhirClient.ReadWithContext(ctx, "Patient", &patientAndPractitionerBundle, fhirclient.QueryParam("_include", "Patient:general-practitioner")); err != nil {
		return nil, fmt.Errorf("unable to fetch Patient and Practitioner bundle: %w", err)
	}
	var patient fhir.Patient
	if err := coolfhir.ResourceInBundle(&patientAndPractitionerBundle, coolfhir.EntryIsOfType("Patient"), &patient); err != nil {
		return nil, fmt.Errorf("unable to find Patient resource in Bundle: %w", err)
	}

	// var conditionBundle fhir.Bundle
	//TODO: We assume this is in context as the HCP token is workflow-specific. Double-check with Zorgplatform
	// var conditions []map[string]interface{}
	// if err = fhirClient.ReadWithContext(ctx, "Condition", &conditionBundle, fhirclient.QueryParam("subject", "Patient/"+*patient.Id)); err != nil {
	// 	return nil, fmt.Errorf("unable to fetch Conditions bundle: %w", err)
	// }
	// if err := coolfhir.ResourcesInBundle(&conditionBundle, coolfhir.EntryIsOfType("Condition"), &conditions); err != nil {
	// 	return nil, fmt.Errorf("unable to find Condition resources in Bundle: %w", err)
	// }

	// if *conditionBundle.Total < 1 {
	// 	return nil, fmt.Errorf("expected at least one Condition, got %d", conditionBundle.Total)
	// }

	var reasonReference fhir.Reference

	reason := fhir.Condition{
		Id:   to.Ptr(uuid.NewString()),
		Code: conditionCode,
	}

	ref := "Condition/magic-" + *reason.Id
	reasonReference = fhir.Reference{
		Type:      to.Ptr("Condition"),
		Reference: to.Ptr(ref),
		Display:   to.Ptr(*reason.Code.Text),
	}

	// Resolve identity of local care organization
	identities, err := s.profile.Identities(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch identities: %w", err)
	}
	if len(identities) != 1 {
		return nil, fmt.Errorf("expected exactly one identity, got %d", len(identities))
	}
	organization := identities[0]

	for _, identifier := range patient.Identifier {
		if identifier.System != nil && *identifier.System == coolfhir.BSNNamingSystem {
			//TODO: overwriting the BSN with the one from the Patient resource, as with test data they can differ, normally we want to throw an error
			launchContext.Bsn = *identifier.Value
			break
		}
	}

	// Zorgplatform does not provide a ServiceRequest, so we need to create one based on other resources they do use
	taskPerformer := fhir.Reference{
		Identifier: &fhir.Identifier{
			System: to.Ptr(coolfhir.URANamingSystem),
			Value:  &s.config.TaskPerformerUra,
		},
	}
	// Enrich performer URA with registered name
	if result, err := s.profile.CsdDirectory().LookupEntity(ctx, *taskPerformer.Identifier); err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Couldn't resolve performer name (ura: %s)", s.config.TaskPerformerUra)
	} else {
		taskPerformer = *result
	}

	serviceRequest := &fhir.ServiceRequest{
		Status: fhir.RequestStatusActive,
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr(zorgplatformWorkflowIdSystem),
				Value:  to.Ptr(launchContext.WorkflowId),
			},
		},
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				// Hardcoded, we only do Telemonitoring for now
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("719858009"),
					Display: to.Ptr("monitoren via telegeneeskunde"),
				},
			},
		},
		ReasonReference: []fhir.Reference{reasonReference},
		Subject: fhir.Reference{
			Type: to.Ptr("Patient"),
			Identifier: &fhir.Identifier{
				System: to.Ptr(coolfhir.BSNNamingSystem),
				Value:  &launchContext.Bsn,
			},
		},
		Performer: []fhir.Reference{taskPerformer},
		Requester: &fhir.Reference{
			Identifier: &organization.Identifier[0],
		},
	}

	sessionData := session.Data{
		FHIRLauncher:   launcherKey,
		TaskIdentifier: to.Ptr(zorgplatformWorkflowIdSystem + "|" + launchContext.WorkflowId),
		LauncherProperties: map[string]string{
			"accessToken": accessToken,
		},
		//LaunchContext: launchContext, // Can be used to fetch a new access token after expiration
	}
	sessionData.Set("Patient/"+*patient.Id, patient)
	sessionData.Set("ServiceRequest/magic-"+uuid.NewString(), *serviceRequest)
	sessionData.Set("Practitioner/magic-"+uuid.NewString(), launchContext.Practitioner)
	sessionData.Set("PractitionerRole/magic-"+uuid.NewString(), launchContext.PractitionerRole)
	sessionData.Set("Organization/magic-"+uuid.NewString(), organization)
	sessionData.Set(*reasonReference.Reference, reason)
	if existingTaskRef != nil {
		sessionData.Set(*existingTaskRef, nil)
	}
	return &sessionData, nil
}

func (s *Service) cpsFhirClient() fhirclient.Client {
	return globals.CarePlanServiceFhirClient
}

// getConditionCodeFromWorkflowTask returns a CodeableConcept based on the workflow definition reference of the Task.
// The workflow definition reference can be in the following formats:
// - ActivityDefinition/urn:oid:1.2.3.4
// - ActivityDefinition/1.2.3.4
// - ActivityDefinition/1.0 (test case of Zorgplatform Developer Portal)
func getConditionCodeFromWorkflowTask(task map[string]interface{}) (*fhir.CodeableConcept, error) {
	var workflowReference string
	if definitionRef, ok := task["definitionReference"].(map[string]interface{}); !ok {
		return nil, fmt.Errorf("Task.definitionReference is missing or invalid")
	} else if workflowReference, ok = definitionRef["reference"].(string); !ok {
		return nil, fmt.Errorf("Task.definitionReference.reference is missing or invalid")
	}
	prefix := "ActivityDefinition/"
	if !strings.HasPrefix(workflowReference, prefix) {
		return nil, fmt.Errorf("Task.definitionReference.reference does is not in the form '%s/<id>': %s", prefix, workflowReference)
	}
	// Mapping defined by https://github.com/Zorgbijjou/oid-repository/blob/main/oid-repository.md
	activityId := strings.TrimPrefix(workflowReference, prefix)
	activityId = strings.TrimPrefix(activityId, "urn:oid:")
	switch activityId {
	case "1.0":
		// Used by Zorgplatform Developer Portal, default to Hartfalen
		fallthrough
	case "2.16.840.1.113883.2.4.3.224.2.1":
		return &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("84114007"),
					Display: to.Ptr("hartfalen"),
				},
			},
			Text: to.Ptr("hartfalen"),
		}, nil
	case "2.16.840.1.113883.2.4.3.224.2.2":
		return &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("13645005"),
					Display: to.Ptr("chronische obstructieve longaandoening"),
				},
			},
			Text: to.Ptr("chronische obstructieve longaandoening"),
		}, nil
	case "2.16.840.1.113883.2.4.3.224.2.3":
		return &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("195967001"),
					Display: to.Ptr("astma"),
				},
			},
			Text: to.Ptr("astma"),
		}, nil
	}
	return nil, fmt.Errorf("unsupported workflow definition: %s", workflowReference)
}

type authHeaderRoundTripper struct {
	value string
	inner http.RoundTripper
}

func (a authHeaderRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.Header.Add("Authorization", a.value)
	return a.inner.RoundTrip(request)
}

func getCertificates(ctx context.Context, pemCert string) ([]*x509.Certificate, error) {
	if len(pemCert) == 0 {
		return nil, errors.New("no Zorgplatform signing certificate configured")
	}
	block, rest := pem.Decode([]byte(pemCert))
	if block == nil && len(rest) > 0 {
		if globals.StrictMode {
			return nil, errors.New("failed to decode certificate PEM")
		}
		// Fallback to hardcoded certificate - for some reason the certificate cannot be decoded on Docker Desktop OSX from the env var
		log.Warn().Msg("Zorgplatform certificate could not be decoded by environment variable, falling back to hardcoded certificate")
		pemCert = `-----BEGIN CERTIFICATE-----
MIIGpTCCBY2gAwIBAgIJAJ7SiMwCRCiBMA0GCSqGSIb3DQEBCwUAMIG0MQswCQYD
VQQGEwJVUzEQMA4GA1UECBMHQXJpem9uYTETMBEGA1UEBxMKU2NvdHRzZGFsZTEa
MBgGA1UEChMRR29EYWRkeS5jb20sIEluYy4xLTArBgNVBAsTJGh0dHA6Ly9jZXJ0
cy5nb2RhZGR5LmNvbS9yZXBvc2l0b3J5LzEzMDEGA1UEAxMqR28gRGFkZHkgU2Vj
dXJlIENlcnRpZmljYXRlIEF1dGhvcml0eSAtIEcyMB4XDTI0MDcwMzE2MDMyNFoX
DTI1MDgwNDE2MDMyNFowIDEeMBwGA1UEAwwVKi56b3JncGxhdGZvcm0ub25saW5l
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAptpmGW3pOURCzuF1+oyP
vIW8bGEjPLyRzMfn29WhNFj8HrkH7+tQCaNE3aL1TTcskwAZEsXTxC9pGAbuqGrU
Ukc8TIDX/Ze6W9CnT/bUYfYl43hLXmmdElxdCtYZAcJtbIeR3bSR1hqdH0w3T/Ob
Q8bDl5TXGX6IPPf/vZ/tBsv/886brz+bXSwArSzNwVBqvVW+EpSyWnDm45/oXzbE
N9Sl7kRzlUuzMDH+GWGNRGOItBfIfixRp4NiopBt7WYBw/lOaUvjYH+GsC46fZzT
Xu0gwzkw8/AqJRyS0OkYGmddEswUizBIPH6OLmjPskpqc6WrsoL2VipcaA+hr0Fz
ZwIDAQABo4IDSzCCA0cwDAYDVR0TAQH/BAIwADAdBgNVHSUEFjAUBggrBgEFBQcD
AQYIKwYBBQUHAwIwDgYDVR0PAQH/BAQDAgWgMDkGA1UdHwQyMDAwLqAsoCqGKGh0
dHA6Ly9jcmwuZ29kYWRkeS5jb20vZ2RpZzJzMS0yNDM0MS5jcmwwXQYDVR0gBFYw
VDBIBgtghkgBhv1tAQcXATA5MDcGCCsGAQUFBwIBFitodHRwOi8vY2VydGlmaWNh
dGVzLmdvZGFkZHkuY29tL3JlcG9zaXRvcnkvMAgGBmeBDAECATB2BggrBgEFBQcB
AQRqMGgwJAYIKwYBBQUHMAGGGGh0dHA6Ly9vY3NwLmdvZGFkZHkuY29tLzBABggr
BgEFBQcwAoY0aHR0cDovL2NlcnRpZmljYXRlcy5nb2RhZGR5LmNvbS9yZXBvc2l0
b3J5L2dkaWcyLmNydDAfBgNVHSMEGDAWgBRAwr0njsw0gzCiM9f7bLPwtCyAzjA1
BgNVHREELjAsghUqLnpvcmdwbGF0Zm9ybS5vbmxpbmWCE3pvcmdwbGF0Zm9ybS5v
bmxpbmUwHQYDVR0OBBYEFPZefQaBIVcTvBQ6Q7aL5Xs9z+OrMIIBfQYKKwYBBAHW
eQIEAgSCAW0EggFpAWcAdgAS8U40vVNyTIQGGcOPP3oT+Oe1YoeInG0wBYTr5YYm
OgAAAZB5Vh7tAAAEAwBHMEUCIQDdEs3O/Bh0XyB/bNCDYHnGsvy2uvIqLGLUyXcI
zi97pwIgWUdyVuJi9r6l0iVFJpNiHIl/7OdG6v7F1ppRsRQ4gFwAdQB9WR4S4Xgq
exxhZ3xe/fjQh1wUoE6VnrkDL9kOjC55uAAAAZB5Vh/1AAAEAwBGMEQCIBZ0Y+G1
jNdhFJXKRwhWkkIhRmCKPuBN/U596oL7Yta7AiAZ9hEqvZw8qqWckQR5M0He2rgF
WE9w3frfzuYNd9OsGAB2AMz7D2qFcQll/pWbU87psnwi6YVcDZeNtql+VMD+TA2w
AAABkHlWIKkAAAQDAEcwRQIhAKI5arrZ02GLep/gElJGSxNJp4HepzjXJC5dF9N7
5et3AiAqQHOYLY1u8xWl45guYPxpBiSKf+bKxhyZYPCN1wRQEzANBgkqhkiG9w0B
AQsFAAOCAQEAGFpFlsmdTCsiSEgwSHW1NPgeZV0EkiS7wz52iuLdphheoIY9xw44
iPNrUknBcP9gfoMpUmMGKelwDdauUitEsHQYo2cFATJvIGyMkK5hxcldZdmjgehi
8tXl7/3gH3R2f6CPOEUbG/+Tlc50cdN0o4jd/qZlfMjDo9odblOVHe4oOlnJYugB
KLh5Cy6PjY6n28xqStJFd2Aximzius46N1XC1XjtMCpwUov+wrf3/CkDTc7dWSU3
yBBl3pbBMYkf2wjOBGWWXcRuK+Tldk1nA0SI0zRRlzjgi4mD74fXdUwtr8Chsh9u
U6OWTXiki5XGd75h6duSZG9qvqymSIuTjA==
-----END CERTIFICATE-----`
	}
	var result []*x509.Certificate
	rest = []byte(pemCert)
	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, fmt.Errorf("failed to decode certificate PEM block #%d", len(result))
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("expected CERTIFICATE block, got %s in PEM block #%d", block.Type, len(result))
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate #%d: %w", len(result), err)
		}
		result = append(result, cert)
		log.Ctx(ctx).Info().Msgf("Successfully loaded Zorgplatform signing certificate, expiry=%s", cert.NotAfter)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no valid certificates found in PEM data")
	}
	return result, nil
}
