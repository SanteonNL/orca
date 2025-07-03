package smartonfhir

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/applaunch/session"
	"github.com/SanteonNL/orca/orchestrator/lib/az/azkeyvault"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
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
			require.Equal(t, []string{"openid fhirUser user/Patient.r user/Practitioner.r launch"}, capturedScope)
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
	privateKey, _ := rsa.GenerateKey(rand.Reader, 1024)
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
}
