package smartonfhir

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestService(t *testing.T) {
	httpMux := http.NewServeMux()
	httpServer := httptest.NewServer(httpMux)
	sessionManager := user.NewSessionManager[session.Data](time.Minute)

	// Set up client
	const clientID = "test-client-id"
	clientURL := must.ParseURL(httpServer.URL).JoinPath("smart-app-launch")
	var frontendCalled bool
	httpMux.HandleFunc("/frontend/list", func(writer http.ResponseWriter, request *http.Request) {
		frontendCalled = true
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte("<html><body>Frontend called</body></html>"))
	})

	// Set up SMART on FHIR issuer
	issuerURL := httpServer.URL + "/fhir"
	httpMux.HandleFunc("/fhir/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issuer": "` + issuerURL + `",
			"authorization_endpoint": "` + issuerURL + `/authorize",
			"token_endpoint": "` + issuerURL + `/token",
			"jwks_uri": "` + issuerURL + `/keys"
		}`))
	})
	issuerPrivateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	issuerSigner, err := jose.NewSigner(jose.SigningKey{
		Algorithm: "ES256",
		Key:       issuerPrivateKey,
	}, &jose.SignerOptions{})
	require.NoError(t, err)
	httpMux.HandleFunc("/fhir/keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{
			Keys: []jose.JSONWebKey{
				{
					Key:       issuerPrivateKey.Public(),
					KeyID:     "default",
					Algorithm: "ES256",
					Use:       "sig",
				},
			},
		})
	})
	const fhirPatientID = "1234"
	httpMux.HandleFunc("/fhir/Patient/"+fhirPatientID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = w.Write([]byte(`{
			"resourceType": "Patient",
			"id": "` + fhirPatientID + `",
			"name": [{"family": "Doe", "given": ["John"]}]
		}`))
	})
	const fhirPractitionerID = "7890"
	httpMux.HandleFunc("/fhir/Practitioner/"+fhirPractitionerID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = w.Write([]byte(`{
			"resourceType": "Practitioner",
			"id": "` + fhirPractitionerID + `",
			"name": [{"family": "Smith", "given": ["Jane"]}]
		}`))
	})

	var capturedScope []string
	var capturedAudience []string
	var capturedClientID []string
	var capturedLaunchParam []string
	httpMux.HandleFunc("/fhir/authorize", func(w http.ResponseWriter, r *http.Request) {
		capturedScope = r.URL.Query()["scope"]
		capturedAudience = r.URL.Query()["aud"]
		capturedClientID = r.URL.Query()["client_id"]
		capturedLaunchParam = r.URL.Query()["launch"]
		// Simulate an authorization response
		http.Redirect(w, r, r.URL.Query().Get("redirect_uri")+"?state="+r.URL.Query().Get("state"), http.StatusFound)
	})
	var capturedClientAssertion string
	httpMux.HandleFunc("/fhir/token", func(w http.ResponseWriter, r *http.Request) {
		capturedClientAssertion = r.PostFormValue("client_assertion")
		// Simulate a token response
		idToken, err := jwt.Signed(issuerSigner).
			Claims(jwt.Claims{
				Issuer:    issuerURL,
				Subject:   uuid.NewString(),
				Audience:  jwt.Audience{clientID},
				Expiry:    jwt.NewNumericDate(time.Now().Add(time.Minute)),
				NotBefore: jwt.NewNumericDate(time.Now()),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ID:        uuid.NewString(),
			}).
			Claims(map[string]any{
				"fhirUser": "Practitioner/" + fhirPractitionerID,
			}).
			Serialize()
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")

		_, _ = w.Write([]byte(`{
			"access_token": "test-access-token",
			"id_token": "` + idToken + `",
			"token_type": "Bearer",	
			"expires_in": 3600,
			"scope": "` + strings.Join(capturedScope, " ") + `",
			"patient": "1234"
		}`))
	})

	service, err := New(Config{
		Enabled: true,
		Issuer: map[string]IssuerConfig{
			"test": {URL: issuerURL, ClientID: clientID},
		},
		AzureKeyVault: AzureKeyVaultConfig{},
	}, sessionManager, must.ParseURL(httpServer.URL), must.ParseURL(httpServer.URL).JoinPath("frontend"), false)
	require.NoError(t, err)
	service.RegisterHandlers(httpMux)

	t.Run("JWK set", func(t *testing.T) {
		httpResponse, err := httpServer.Client().Get(clientURL.JoinPath("/.well-known/jwks.json").String())
		require.NoError(t, err)
		defer httpResponse.Body.Close()
		require.Equal(t, http.StatusOK, httpResponse.StatusCode, "Expected status code 200 OK for JWK set")
		responseData, err := io.ReadAll(httpResponse.Body)
		require.NoError(t, err, "Failed to read response body for JWK set")
		keySet := jose.JSONWebKeySet{}
		require.NoError(t, json.Unmarshal(responseData, &keySet))
		require.Len(t, keySet.Keys, 1)
		require.IsType(t, &ecdsa.PublicKey{}, keySet.Keys[0].Key)
		require.Equal(t, "default", keySet.Keys[0].KeyID)
		require.Equal(t, "sig", keySet.Keys[0].Use)
	})

	t.Run("app launch", func(t *testing.T) {
		t.Run("with launch parameter", func(t *testing.T) {
			defer func() {
				frontendCalled = false
			}()
			cookieJar, _ := cookiejar.New(nil)
			httpClient := http.Client{
				Jar: cookieJar,
				Transport: &loggingRoundTripper{
					t: t,
				},
			}

			httpRequestQuery := url.Values{
				"iss":    []string{issuerURL},
				"launch": []string{"test-launch"},
			}
			httpResponse, err := httpClient.Get(clientURL.String() + "?" + httpRequestQuery.Encode())
			require.NoError(t, err)
			responseData, _ := io.ReadAll(httpResponse.Body)
			println(string(responseData))
			require.Equal(t, http.StatusOK, httpResponse.StatusCode, "Expected status code 200 OK for app launch with launch parameter")
			// Assert captured authorization request parameters
			require.Equal(t, []string{"openid fhirUser launch"}, capturedScope)
			require.Equal(t, []string{issuerURL}, capturedAudience)
			require.Equal(t, []string{clientID}, capturedClientID)
			require.Equal(t, []string{"test-launch"}, capturedLaunchParam)
			// Assert the "browser" was redirected to the frontend
			require.True(t, frontendCalled)
			// Assert client_assertion
			require.NotEmpty(t, capturedClientAssertion)
			decodedClientAssertion, err := jwt.ParseSigned(capturedClientAssertion, []jose.SignatureAlgorithm{jose.ES256})
			require.NoError(t, err, "Failed to parse client assertion JWT")
			var claims jwt.Claims
			err = decodedClientAssertion.UnsafeClaimsWithoutVerification(&claims)
			require.NoError(t, err, "Failed to extract claims from client assertion JWT")
			require.Equal(t, clientID, claims.Issuer)
			require.Equal(t, clientID, claims.Subject)
			require.Equal(t, jwt.Audience{issuerURL + "/token"}, claims.Audience)
			require.NotEmpty(t, claims.ID, "Client assertion JWT should have an ID")
		})
		t.Run("without launch parameter", func(t *testing.T) {
			defer func() {
				frontendCalled = false
			}()
			cookieJar, _ := cookiejar.New(nil)
			httpClient := http.Client{
				Jar: cookieJar,
				Transport: &loggingRoundTripper{
					t: t,
				},
			}

			httpRequestQuery := url.Values{
				"iss": []string{issuerURL},
			}
			httpResponse, err := httpClient.Get(clientURL.String() + "?" + httpRequestQuery.Encode())
			require.NoError(t, err)
			responseData, _ := io.ReadAll(httpResponse.Body)
			println(string(responseData))
			require.Equal(t, http.StatusOK, httpResponse.StatusCode, "Expected status code 200 OK for app launch with launch parameter")
			// Assert captured authorization request parameters
			require.Equal(t, []string{issuerURL}, capturedAudience)
			require.Equal(t, []string{clientID}, capturedClientID)
			require.Equal(t, []string(nil), capturedLaunchParam)
			// Assert the "browser" was redirected to the frontend
			require.True(t, frontendCalled)
		})
	})
}

type loggingRoundTripper struct {
	t *testing.T
}

func (l loggingRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	l.t.Log("Making request to:", request.URL.String())
	return http.DefaultTransport.RoundTrip(request)
}

func Test_loadJWTSigningKeyFromAzureKeyVault(t *testing.T) {
	const privateKeyHex = "3082025b02010002818100e4b916c35e9d5495eab5018eb6b2841f8a85fe12cc88dbf18118c300889e6011bd136b6812efa60f199ad27480140625a8e9f4eaef674f4a170fcd9cb591f25baec64874f140050910837f52265ff6204c23371b40180533fd38cfe1231ba21366ae0db311756a5c02586a3eb4e56cc6b7d0b0c6ccca01bf3aa97d2f16d4b3d3020301000102818055c42e0bfeaba26f40feb4e1ce126cc6e30bd1b53ceb49066b715c9613a4c7c37f120c218f903bc5c7c52d8bb0075232c6ff4beed8ecf56783f45216a46360ec58b44ba66cbc8da646e49cece72eb24051b40caf1cb4a13c6bee5fec202b69cf5152fb24af16373e9b406de7122cb8993797827847c88795b4719262bc46da71024100f93a72539d094d7d602416fa969714e27a3fe7b08b848c13f7890bcd76557b3b30db16fc51b7be1f1f165ba9145766066db5907e00062c5760dbef4cc3a136e9024100eaf0038fe1aaea64abda521b4989376c027d7ffd72662379a8d7578be07296076b4ab961ab93103e6748ab9342fd9b2f556bf8f648904be3205837c89bd5575b0240654b07e44bd2d817b8d7722f6ebd00d3fb73b5aadf4983d529aa1d8de8265deb74b3d6e7be1ebbbad25bb7ed444331483396b39f424b4002536f9016d6fbd2e102401d21516fbfad6f8eb7f84401fa263766ff100c94a260a3b96c03f768f29582a0bcdef109793aace2efef84c6a7a1c66222175731426211e6c195eea4c31dbacd02403caead7210df2df81bf8d05231d4480d1cf4731ed2dd13a5a7e76c6fd85af80b2f282dcf0756f16641cf98f59e09196bf61d93b3835caa519185131259a1d06d"
	// Use a stable private key for deterministic tests. It was generated with:
	//privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
	//pkBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	//println(hex.EncodeToString(pkBytes))
	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	require.NoError(t, err)
	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBytes)
	require.NoError(t, err)

	keyVault := azkeyvault.NewTestServer()
	azkeyvault.AzureHttpRequestDoer = keyVault.TestHttpServer.Client()
	keyVault.AddKey("test-key-id", privateKey)

	signingKey, jwkKeySet, err := loadJWTSigningKeyFromAzureKeyVault(AzureKeyVaultConfig{
		URL:            keyVault.TestHttpServer.URL,
		SigningKeyName: "test-key-id",
		CredentialType: "managed_identity",
	}, true)

	require.NoError(t, err)
	require.NotNil(t, signingKey)
	require.NotNil(t, jwkKeySet)

	require.Len(t, jwkKeySet.Keys, 1, "Expected one key in JWK set")
	require.Equal(t, "RS256", jwkKeySet.Keys[0].Algorithm, "Expected key algorithm to be RS256")
	require.Equal(t, "sig", jwkKeySet.Keys[0].Use, "Expected key use to be 'sig'")
	require.Equal(t, "59adb1f2af5539daa47e7053ba82f80685e81c5324a19f6ac27b55f58a7d92ed", jwkKeySet.Keys[0].KeyID, "Expected key ID to be '0'")
	require.NotNil(t, jwkKeySet.Keys[0].Key)
}
