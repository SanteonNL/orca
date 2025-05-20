package zorgplatform

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"hash"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/jellydator/ttlcache/v3"

	"github.com/stretchr/testify/assert"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/beevik/etree"
	"github.com/braineet/saml/xmlenc"
	"github.com/segmentio/asm/base64"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService(t *testing.T) {
	httpServerMux := http.NewServeMux()
	httpServer := httptest.NewServer(httpServerMux)

	ctrl := gomock.NewController(t)
	keysClient := azkeyvault.NewMockKeysClient(ctrl)
	certsClient := azkeyvault.NewMockCertificatesClient(ctrl)

	// Client cert
	// Certificate and key generated with:
	// openssl req -x509 -newkey rsa:2048 -keyout test-key.pem -out test-certificate.pem -sha256 -days 9999 -nodes -subj "/CN=localhost"
	certificate, err := tls.LoadX509KeyPair("test-certificate.pem", "test-key.pem")
	require.NoError(t, err)
	clientCertName := "client-cert"
	certsClient.EXPECT().GetCertificate(gomock.Any(), clientCertName, "", nil).
		Return(azcertificates.GetCertificateResponse{
			Certificate: azcertificates.Certificate{
				CER: certificate.Certificate[0],
				KID: (*azcertificates.ID)(&clientCertName),
			},
		}, nil).MinTimes(1)
	keysClient.EXPECT().GetKey(gomock.Any(), clientCertName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(certificate.PrivateKey.(*rsa.PrivateKey).PublicKey, clientCertName, "0"),
			},
		}, nil).MinTimes(1)
	// Decryption cert
	decryptKeyName := "decrypt-cert"
	keysClient.EXPECT().GetKey(gomock.Any(), decryptKeyName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(*certificate.Leaf.PublicKey.(*rsa.PublicKey), decryptKeyName, "0"),
			},
		}, nil).MinTimes(1)
	keysClient.EXPECT().Decrypt(gomock.Any(), decryptKeyName, "0", gomock.Any(), nil).
		DoAndReturn(func(ctx interface{}, keyName string, keyVersion string, parameters azkeys.KeyOperationParameters, options *azkeys.DecryptOptions) (azkeys.DecryptResponse, error) {
			var h hash.Hash
			switch *parameters.Algorithm {
			case azkeys.EncryptionAlgorithmRSAOAEP:
				h = sha1.New()
			case azkeys.EncryptionAlgorithmRSAOAEP256:
				h = sha256.New()
			default:
				return azkeys.DecryptResponse{}, fmt.Errorf("unexpected algorithm: %s", *parameters.Algorithm)
			}
			result, err := rsa.DecryptOAEP(h, rand.Reader, certificate.PrivateKey.(*rsa.PrivateKey), parameters.Value, nil)
			if err != nil {
				return azkeys.DecryptResponse{}, err
			}
			return azkeys.DecryptResponse{
				KeyOperationResult: azkeys.KeyOperationResult{
					Result: result,
				},
			}, nil
		}).MinTimes(1)
	// Signing cert
	signKeyName := "sign-cert"
	signingKeyPair, _ := rsa.GenerateKey(rand.Reader, 2048)
	certsClient.EXPECT().GetCertificate(gomock.Any(), signKeyName, "", nil).
		Return(azcertificates.GetCertificateResponse{
			Certificate: azcertificates.Certificate{
				CER: certificate.Certificate[0],
				KID: (*azcertificates.ID)(&signKeyName),
			},
		}, nil).MinTimes(1)
	keysClient.EXPECT().GetKey(gomock.Any(), signKeyName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(signingKeyPair.PublicKey, signKeyName, "0"),
			},
		}, nil).MinTimes(1)

	zorgplatformHttpServerMux := http.NewServeMux()
	zorgplatformHttpServer := httptest.NewServer(zorgplatformHttpServerMux)
	zorgplatformHttpServerMux.Handle("GET /api/Task/b526e773-e1a6-4533-bd00-1360c97e745f", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Zorgplatform Workflow Task
		coolfhir.SendResponse(w, http.StatusOK, map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/1.0",
			},
		})
	}))
	zorgplatformHttpServerMux.Handle("GET /api/Patient", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coolfhir.SendResponse(w, http.StatusOK, coolfhir.SearchSet().
			Append(
				fhir.Patient{
					Id: to.Ptr("pat-123"),
				}, nil, nil).
			Append(
				fhir.PractitionerRole{
					Practitioner: &fhir.Reference{
						Reference: to.Ptr("Practitioner/prac-123"),
					},
				}, nil, nil).
			Append(
				fhir.Practitioner{
					Id: to.Ptr("prac-123"),
				}, nil, nil),
		)
	}))

	certDER := certificate.Certificate[0]

	// Encode to PEM
	pemBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	cfg := Config{
		Enabled:            true,
		ApiUrl:             zorgplatformHttpServer.URL + "/api",
		SAMLRequestTimeout: 10 * time.Second,
		SigningConfig: SigningConfig{
			Issuer:   "https://partner-application.nl",
			Audience: "unit-test",
		},
		DecryptConfig: DecryptConfig{
			Issuer:      "unit-test",
			Audience:    "https://partner-application.nl",
			SignCertPem: string(pemBlock),
		},
		AzureConfig: AzureConfig{
			CredentialType: "default",
			KeyVaultConfig: AzureKeyVaultConfig{
				DecryptCertName: decryptKeyName,
				SignCertName:    signKeyName,
				ClientCertName:  clientCertName,
				AllowInsecure:   true,
			},
		},
	}

	sessionManager := user.NewSessionManager[session.Data](time.Minute)
	service, err := newWithClients(context.Background(), sessionManager, cfg, httpServer.URL, must.ParseURL("/frontend"), keysClient, certsClient, profile.Test())
	require.NoError(t, err)
	service.secureTokenService = &stubSecureTokenService{}
	service.RegisterHandlers(httpServerMux)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	t.Run("ok, new Task", func(t *testing.T) {
		globals.CarePlanServiceFhirClient = &test.StubFHIRClient{}

		launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
			"SAMLResponse": {createSAMLResponse(t, certificate.Leaf)},
		})

		require.NoError(t, err)
		require.Equal(t, http.StatusFound, launchHttpResponse.StatusCode)
		require.Equal(t, "/frontend/new", launchHttpResponse.Header.Get("Location"))

		t.Run("assert user session", func(t *testing.T) {
			sessionData := user.SessionFromHttpResponse(sessionManager, launchHttpResponse)
			require.NotNil(t, sessionData)

			t.Run("check Practitioner is in session", func(t *testing.T) {
				assert.NotNil(t, session.Get[fhir.Practitioner](sessionData))
			})
			t.Run("check ServiceRequest is in session", func(t *testing.T) {
				serviceRequest := session.Get[fhir.ServiceRequest](sessionData)
				require.NotNil(t, serviceRequest)
				t.Run("check Workflow-ID identifier is properly set on the ServiceRequest", func(t *testing.T) {
					assert.Contains(t, serviceRequest.Identifier, fhir.Identifier{
						System: to.Ptr("http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
						Value:  to.Ptr("b526e773-e1a6-4533-bd00-1360c97e745f"),
					})
				})
			})
			t.Run("check Patient is in session", func(t *testing.T) {
				assert.NotNil(t, session.Get[fhir.Patient](sessionData))
			})
		})
	})
	t.Run("ok, existing Task", func(t *testing.T) {
		existingTask := fhir.Task{
			Id: to.Ptr("12345678910"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
					Value:  to.Ptr("b526e773-e1a6-4533-bd00-1360c97e745f"),
				},
			},
		}
		cpsFHIRClient := &test.StubFHIRClient{
			Resources: []interface{}{
				existingTask,
			},
		}
		globals.CarePlanServiceFhirClient = cpsFHIRClient

		launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
			"SAMLResponse": {createSAMLResponse(t, certificate.Leaf)},
		})
		require.NoError(t, err)

		sessionData := user.SessionFromHttpResponse(sessionManager, launchHttpResponse)
		require.NotNil(t, sessionData)
		require.Equal(t, "/frontend/task/12345678910", launchHttpResponse.Header.Get("Location"))
		assert.Equal(t, "Task/"+*existingTask.Id, sessionData.GetByType("Task").Path)
	})

	t.Run("test invalid SAML response", func(t *testing.T) {
		invalidSAMLResponse := "invalidSAMLResponse"
		launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
			"SAMLResponse": {invalidSAMLResponse},
		})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, launchHttpResponse.StatusCode)
	})

	t.Run("test missing SAML response", func(t *testing.T) {
		launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{})
		require.NoError(t, err)
		require.Equal(t, http.StatusBadRequest, launchHttpResponse.StatusCode)
	})

	t.Run("retry getSessionData 3 times - should work", func(t *testing.T) {
		globals.CarePlanServiceFhirClient = &test.StubFHIRClient{}

		// Save the original sleep function and restore it after the test.
		originalSleep := sleep

		// Override sleep to collect the duration of the sleep requested duration instead of actually sleeping.
		var sleeps []time.Duration
		sleep = func(t time.Duration) { sleeps = append(sleeps, t) }

		// Mock getSessionData to fail twice before succeeding
		callCount := 0
		service.getSessionData = func(ctx context.Context, accessToken string, launchContext LaunchContext) (*session.Data, error) {
			callCount++
			if callCount < 3 {
				return nil, errors.New("temporary error")
			}
			return service.defaultGetSessionData(ctx, accessToken, launchContext)
		}

		launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
			"SAMLResponse": {createSAMLResponse(t, certificate.Leaf)},
		})

		require.NoError(t, err)
		require.Equal(t, http.StatusFound, launchHttpResponse.StatusCode)
		require.Equal(t, 3, callCount, "expected getSessionData to be called 3 times")
		require.Equal(t, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}, sleeps, "expected 2 incremental sleeps")

		t.Cleanup(func() { sleep = originalSleep })
		t.Cleanup(func() { service.getSessionData = service.defaultGetSessionData })
	})

	t.Run("Consistent error - should fail after 3 retries", func(t *testing.T) {
		globals.CarePlanServiceFhirClient = &test.StubFHIRClient{}

		// Save the original sleep function and restore it after the test.
		originalSleep := sleep

		// Override sleep to collect the duration of the sleep requested duration instead of actually sleeping.
		var sleeps []time.Duration
		sleep = func(t time.Duration) { sleeps = append(sleeps, t) }

		callCount := 0
		service.getSessionData = func(ctx context.Context, accessToken string, launchContext LaunchContext) (*session.Data, error) {
			callCount++
			return nil, errors.New("temporary error") //always throw an error
		}

		launchHttpResponse, _ := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
			"SAMLResponse": {createSAMLResponse(t, certificate.Leaf)},
		})

		require.Equal(t, http.StatusInternalServerError, launchHttpResponse.StatusCode)
		require.Equal(t, 4, callCount, "expected getSessionData to be called 4 times")
		require.Equal(t, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond, 600 * time.Millisecond}, sleeps, "expected 3 incremental sleeps")

		t.Cleanup(func() { sleep = originalSleep })
		t.Cleanup(func() { service.getSessionData = service.defaultGetSessionData })
	})

}

func createSAMLResponse(t *testing.T, encryptionKey *x509.Certificate) string {
	//TODO: This needs to be fixed once the validation verified expiration date
	plainText, err := os.ReadFile("saml_assertion_input.xml") //DO NOT modify this file, it is a signed assertion with the test-certificate.pem via the service integration test - needs to be to validate the signature
	require.NoError(t, err)

	e := xmlenc.OAEP()
	e.BlockCipher = xmlenc.AES256CBC
	e.DigestMethod = &xmlenc.SHA1

	el, err := e.Encrypt(encryptionKey, plainText, nil)

	require.NoError(t, err)
	samlResponse := etree.NewDocument()
	err = samlResponse.ReadFromFile("saml_response_input.xml")
	require.NoError(t, err)

	// add encrypted assertion to RequestedSecurityToken element
	requestedSecurityToken := samlResponse.FindElement("//trust:RequestedSecurityToken/EncryptedAssertion")
	requestedSecurityToken.AddChild(el)

	samlResponseString, err := samlResponse.WriteToString()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString([]byte(samlResponseString))
}

func publicKeyToJWK(key rsa.PublicKey, id string, version string) *azkeys.JSONWebKey {
	kty := azkeys.KeyTypeRSA
	e := binary.BigEndian.AppendUint64(nil, uint64(key.E))
	return &azkeys.JSONWebKey{
		Kty: &kty,
		N:   key.N.Bytes(),
		E:   e,
		KID: (*azkeys.ID)(to.Ptr("https://myvaultname.vault.azure.net/keys/" + id + "/" + version)),
	}
}

var _ SecureTokenService = &stubSecureTokenService{}

type stubSecureTokenService struct {
	invocations int
	accessToken string
}

func (s *stubSecureTokenService) RequestAccessToken(ctx context.Context, launchContext LaunchContext, tokenType TokenType) (string, error) {
	s.invocations++
	if s.accessToken == "" {
		return "stub-at", nil
	}
	return s.accessToken, nil
}

func Test_getConditionCodeFromWorkflowTask(t *testing.T) {
	t.Run("Test code 1.0", func(t *testing.T) {
		task := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/1.0",
			},
		}
		conditionCode, err := getConditionCodeFromWorkflowTask(task)
		require.NoError(t, err)
		require.Len(t, conditionCode.Coding, 1)
		require.Equal(t, "http://snomed.info/sct", *conditionCode.Coding[0].System)
		require.Equal(t, "84114007", *conditionCode.Coding[0].Code)
		require.Equal(t, "hartfalen", *conditionCode.Coding[0].Display)
	})
	t.Run("Heart failure", func(t *testing.T) {
		taskWithOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/urn:oid:2.16.840.1.113883.2.4.3.224.2.1",
			},
		}
		taskWithoutOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/2.16.840.1.113883.2.4.3.224.2.1",
			},
		}
		for _, curr := range []map[string]interface{}{taskWithOid, taskWithoutOid} {
			conditionCode, err := getConditionCodeFromWorkflowTask(curr)
			require.NoError(t, err)
			require.Len(t, conditionCode.Coding, 1)
			require.Equal(t, "http://snomed.info/sct", *conditionCode.Coding[0].System)
			require.Equal(t, "84114007", *conditionCode.Coding[0].Code)
			require.Equal(t, "hartfalen", *conditionCode.Coding[0].Display)
			require.Equal(t, "hartfalen", *conditionCode.Text)
		}
	})
	t.Run("COPD", func(t *testing.T) {
		taskWithOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/urn:oid:2.16.840.1.113883.2.4.3.224.2.2",
			},
		}
		taskWithoutOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/2.16.840.1.113883.2.4.3.224.2.2",
			},
		}
		for _, curr := range []map[string]interface{}{taskWithOid, taskWithoutOid} {
			conditionCode, err := getConditionCodeFromWorkflowTask(curr)
			require.NoError(t, err)
			require.Len(t, conditionCode.Coding, 1)
			require.Equal(t, "http://snomed.info/sct", *conditionCode.Coding[0].System)
			require.Equal(t, "13645005", *conditionCode.Coding[0].Code)
			require.Equal(t, "chronische obstructieve longaandoening", *conditionCode.Coding[0].Display)
			require.Equal(t, "chronische obstructieve longaandoening", *conditionCode.Text)
		}
	})
	t.Run("Asthma", func(t *testing.T) {
		taskWithOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/urn:oid:2.16.840.1.113883.2.4.3.224.2.3",
			},
		}
		taskWithoutOid := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/2.16.840.1.113883.2.4.3.224.2.3",
			},
		}
		for _, curr := range []map[string]interface{}{taskWithOid, taskWithoutOid} {
			conditionCode, err := getConditionCodeFromWorkflowTask(curr)
			require.NoError(t, err)
			require.Len(t, conditionCode.Coding, 1)
			require.Equal(t, "http://snomed.info/sct", *conditionCode.Coding[0].System)
			require.Equal(t, "195967001", *conditionCode.Coding[0].Code)
			require.Equal(t, "astma", *conditionCode.Coding[0].Display)
			require.Equal(t, "astma", *conditionCode.Text)
		}
	})
	t.Run("unknown workflow", func(t *testing.T) {
		task := map[string]interface{}{
			"definitionReference": map[string]interface{}{
				"reference": "ActivityDefinition/other",
			},
		}
		conditionCode, err := getConditionCodeFromWorkflowTask(task)
		require.EqualError(t, err, "unsupported workflow definition: ActivityDefinition/other")
		require.Nil(t, conditionCode)
	})
}

func TestSTSAccessTokenRoundTripper(t *testing.T) {
	tests := []struct {
		name              string
		carePlanReference string
		expectedError     error
		expectedCacheHits int
		carePlan          *fhir.CarePlan
		task              *fhir.Task
		serviceRequest    *fhir.ServiceRequest
		patient           *fhir.Patient
	}{
		{
			name:              "ok",
			carePlanReference: "http://example.com/CarePlan/123",
		},
		{
			name:              "access token cache hit",
			carePlanReference: "http://example.com/CarePlan/123",
			expectedCacheHits: 3,
		},
		{
			name:          "missing X-SCP-Context header",
			expectedError: errors.New("missing X-Scp-Context header"),
		},
		{
			name: "ServiceRequest doesn't have Zorgplatform Workflow ID",
			serviceRequest: &fhir.ServiceRequest{
				Id: to.Ptr("123"),
			},
			carePlanReference: "http://example.com/CarePlan/123",
			expectedError:     errors.New("unable to get workflowId for CarePlan reference: expected ServiceRequest to have 1 identifier with system http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			cpsFHIRClient := &test.StubFHIRClient{}
			if tt.carePlan == nil {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, fhir.CarePlan{
					Id: to.Ptr("123"),
					Subject: fhir.Reference{
						Reference: to.Ptr("Patient/123"),
					},
					Activity: []fhir.CarePlanActivity{
						{
							Reference: &fhir.Reference{
								Reference: to.Ptr("Task/123"),
							},
						},
					},
				})
			} else {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, *tt.carePlan)
			}
			if tt.task == nil {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, fhir.Task{
					Id: to.Ptr("123"),
					Focus: &fhir.Reference{
						Reference: to.Ptr("ServiceRequest/123"),
					},
				})
			} else {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, *tt.task)
			}
			if tt.serviceRequest == nil {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, fhir.ServiceRequest{
					Id: to.Ptr("123"),
					Identifier: []fhir.Identifier{
						{
							System: to.Ptr("http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
							Value:  to.Ptr("415FD2C0-E88D-4C89-B9D6-8FBE31E2D1C9"),
						},
					},
				})
			} else {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, *tt.serviceRequest)
			}
			if tt.patient == nil {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, fhir.Patient{
					Id: to.Ptr("123"),
					Identifier: []fhir.Identifier{
						{
							System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
							Value:  to.Ptr("123456789"),
						},
					},
				})
			} else {
				cpsFHIRClient.Resources = append(cpsFHIRClient.Resources, *tt.patient)
			}

			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				coolfhir.SendResponse(w, http.StatusOK, fhir.Bundle{})
			}))
			rt := stsAccessTokenRoundTripper{
				transport: httpServer.Client().Transport,
				cpsFhirClient: func() fhirclient.Client {
					return cpsFHIRClient
				},
				secureTokenService: &stubSecureTokenService{},
				accessTokenCache: ttlcache.New[string, string](
					ttlcache.WithTTL[string, string](accessTokenCacheTTL),
				),
			}

			for i := 0; i < tt.expectedCacheHits+1; i++ {
				httpRequest := httptest.NewRequest("POST", httpServer.URL, nil)
				httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				httpRequest.Header.Set("X-SCP-Context", tt.carePlanReference)
				httpResponse, err := rt.RoundTrip(httpRequest)
				if tt.expectedError != nil {
					require.EqualError(t, err, tt.expectedError.Error())
				} else {
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, httpResponse.StatusCode)
				}
			}
			assert.Equal(t, tt.expectedCacheHits, int(rt.accessTokenCache.Metrics().Hits))
		})
	}
}

func TestService_EhrFhirProxy(t *testing.T) {
	t.Run("POST-based search is rewritten to GET-based search", func(t *testing.T) {
		cpsHttpServer := setupCarePlanService(t)
		carePlanUrl := cpsHttpServer.URL + "/fhir/" + testCarePlanReference
		zorgplatformFHIRServerMux := http.NewServeMux()
		var actualQueryParams url.Values
		zorgplatformFHIRServerMux.HandleFunc("GET /fhir/Condition", func(w http.ResponseWriter, r *http.Request) {
			actualQueryParams = r.URL.Query()
			coolfhir.SendResponse(w, http.StatusOK, fhir.Bundle{})
		})
		zorgplatformFHIRServer := httptest.NewServer(zorgplatformFHIRServerMux)

		service := &Service{
			zorgplatformHttpClient: zorgplatformFHIRServer.Client(),
			secureTokenService:     &stubSecureTokenService{},
			accessTokenCache: ttlcache.New[string, string](
				ttlcache.WithTTL[string, string](accessTokenCacheTTL),
			),
			config: Config{ApiUrl: zorgplatformFHIRServer.URL + "/fhir"},
		}

		expectedSearchParams := url.Values{
			"_id": {"123"},
		}
		httpRequest := httptest.NewRequest("POST", "/cpc/fhir/Condition/_search", strings.NewReader(expectedSearchParams.Encode()))
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		httpRequest.Header.Set("X-SCP-Context", carePlanUrl)
		httpResponse := httptest.NewRecorder()
		service.EhrFhirProxy().ServeHTTP(httpResponse, httpRequest)

		require.Equal(t, http.StatusOK, httpResponse.Code)
		require.Equal(t, expectedSearchParams, actualQueryParams, "expected search parameters to be passed through")
	})
}

const testCarePlanReference = "CarePlan/CP-1"

func setupCarePlanService(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /fhir/"+testCarePlanReference, func(w http.ResponseWriter, r *http.Request) {
		coolfhir.SendResponse(w, http.StatusOK, fhir.CarePlan{
			Id: to.Ptr("CP-1"),
			Subject: fhir.Reference{
				Reference: to.Ptr("Patient/P-1"),
			},
			Activity: []fhir.CarePlanActivity{
				{
					Reference: &fhir.Reference{
						Reference: to.Ptr("Task/T-1"),
					},
				},
			},
		})
	})
	mux.HandleFunc("GET /fhir/Patient/P-1", func(w http.ResponseWriter, r *http.Request) {
		coolfhir.SendResponse(w, http.StatusOK, fhir.Patient{
			Id: to.Ptr("P-1"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("123456789"),
				},
			},
		})
	})
	mux.HandleFunc("GET /fhir/Task/T-1", func(w http.ResponseWriter, r *http.Request) {
		coolfhir.SendResponse(w, http.StatusOK, fhir.Task{
			Id: to.Ptr("T-1"),
			Focus: &fhir.Reference{
				Reference: to.Ptr("ServiceRequest/SR-1"),
			},
		})
	})
	mux.HandleFunc("GET /fhir/ServiceRequest/SR-1", func(w http.ResponseWriter, r *http.Request) {
		coolfhir.SendResponse(w, http.StatusOK, fhir.ServiceRequest{
			Id: to.Ptr("SR-1"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://sts.zorgplatform.online/ws/claims/2017/07/workflow/workflow-id"),
					Value:  to.Ptr("workflow-id-1"),
				},
			},
		})
	})
	httpServer := httptest.NewServer(mux)
	globals.CarePlanServiceFhirClient = fhirclient.New(must.ParseURL(httpServer.URL).JoinPath("fhir"), http.DefaultClient, nil)
	return httpServer
}
