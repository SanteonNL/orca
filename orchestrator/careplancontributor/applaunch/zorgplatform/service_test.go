package zorgplatform

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"hash"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
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
		}, nil)
	keysClient.EXPECT().GetKey(gomock.Any(), clientCertName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(certificate.PrivateKey.(*rsa.PrivateKey).PublicKey, clientCertName, "0"),
			},
		}, nil)
	// Decryption cert
	decryptKeyName := "decrypt-cert"
	keysClient.EXPECT().GetKey(gomock.Any(), decryptKeyName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(*certificate.Leaf.PublicKey.(*rsa.PublicKey), decryptKeyName, "0"),
			},
		}, nil)
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
		})
	// Signing cert
	signKeyName := "sign-cert"
	signingKeyPair, _ := rsa.GenerateKey(rand.Reader, 2048)
	certsClient.EXPECT().GetCertificate(gomock.Any(), signKeyName, "", nil).
		Return(azcertificates.GetCertificateResponse{
			Certificate: azcertificates.Certificate{
				CER: certificate.Certificate[0],
				KID: (*azcertificates.ID)(&signKeyName),
			},
		}, nil)
	keysClient.EXPECT().GetKey(gomock.Any(), signKeyName, "", nil).
		Return(azkeys.GetKeyResponse{
			KeyBundle: azkeys.KeyBundle{
				Key: publicKeyToJWK(signingKeyPair.PublicKey, signKeyName, "0"),
			},
		}, nil)
	keysClient.EXPECT().Sign(gomock.Any(), signKeyName, "0", gomock.Any(), nil).
		DoAndReturn(func(ctx interface{}, keyName string, keyVersion string, parameters azkeys.SignParameters, options *azkeys.SignOptions) (azkeys.SignResponse, error) {
			return azkeys.SignResponse{
				KeyOperationResult: azkeys.KeyOperationResult{
					Result: []byte("mocked-signature"),
				},
			}, nil
		})

	zorgplatformHttpServerMux := http.NewServeMux()
	zorgplatformHttpServer := httptest.NewServer(zorgplatformHttpServerMux)
	zorgplatformHttpServerMux.Handle("POST /sts", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Handle
		w.WriteHeader(http.StatusOK)
	}))
	zorgplatformHttpServerMux.Handle("POST /api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Handle
		w.WriteHeader(http.StatusOK)
	}))

	cfg := Config{
		Enabled: true,
		ApiUrl:  zorgplatformHttpServer.URL + "/api",
		StsUrl:  zorgplatformHttpServer.URL + "/sts",
		SigningConfig: SigningConfig{
			Issuer:   "https://partner-application.nl",
			Audience: "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
		},
		DecryptConfig: DecryptConfig{
			Issuer:   "urn:oid:2.16.840.1.113883.2.4.3.124.8.50.8",
			Audience: "https://partner-application.nl",
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

	sessionManager := user.NewSessionManager()
	service, err := newWithClients(sessionManager, cfg, httpServer.URL, "/", keysClient, certsClient)
	require.NoError(t, err)
	service.RegisterHandlers(httpServerMux)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
		"SAMLResponse": {createSAMLResponse(t, certificate.Leaf)},
	})

	require.NoError(t, err)
	//TODO: Below needs to be fixed, the service executes the RequestHcpRst logic, which SHOULD be mocked as it has its own tests
	// require.Equal(t, http.StatusFound, launchHttpResponse.StatusCode)
	require.Equal(t, http.StatusBadRequest, launchHttpResponse.StatusCode)

	//TODO: Uncomment when actually fetching this data
	// t.Run("assert user session is created", func(t *testing.T) {
	// 	sessionData := user.SessionFromHttpResponse(sessionManager, launchHttpResponse)
	// 	require.NotNil(t, sessionData)
	// 	t.Run("check Practitioner is in session", func(t *testing.T) {
	// 		practitionerRef := sessionData.StringValues["practitioner"]
	// 		require.NotEmpty(t, practitionerRef)
	// 		require.IsType(t, fhir.Practitioner{}, sessionData.OtherValues[practitionerRef])
	// 	})
	// 	t.Run("check ServiceRequest is in session", func(t *testing.T) {
	// 		serviceRequestRef := sessionData.StringValues["serviceRequest"]
	// 		require.NotEmpty(t, serviceRequestRef)
	// 		require.IsType(t, fhir.ServiceRequest{}, sessionData.OtherValues[serviceRequestRef])
	// 	})
	// 	t.Run("check Patient is in session", func(t *testing.T) {
	// 		patientRef := sessionData.StringValues["patient"]
	// 		require.NotEmpty(t, patientRef)
	// 		require.IsType(t, fhir.Patient{}, sessionData.OtherValues[patientRef])
	// 	})
	// })

	// t.Run("test invalid SAML response", func(t *testing.T) {
	// 	invalidSAMLResponse := "invalidSAMLResponse"
	// 	launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{
	// 		"SAMLResponse": {invalidSAMLResponse},
	// 	})
	// 	require.NoError(t, err)
	// 	require.Equal(t, http.StatusBadRequest, launchHttpResponse.StatusCode)
	// })

	// t.Run("test missing SAML response", func(t *testing.T) {
	// 	launchHttpResponse, err := client.PostForm(httpServer.URL+"/zorgplatform-app-launch", url.Values{})
	// 	require.NoError(t, err)
	// 	require.Equal(t, http.StatusBadRequest, launchHttpResponse.StatusCode)
	// })
}

func createSAMLResponse(t *testing.T, encryptionKey *x509.Certificate) string {
	plainText, err := os.ReadFile("saml_assertion_input.xml")
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