package nuts

import (
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/nuts-foundation/go-did/vc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDutchNutsProfile_RegisterHTTPHandlers(t *testing.T) {
	const basePath = "/basement"
	var baseURL, _ = url.Parse("http://example.com" + basePath)
	serverMux := http.NewServeMux()
	DutchNutsProfile{}.RegisterHTTPHandlers("/basement", baseURL, serverMux)
	server := httptest.NewServer(serverMux)

	httpResponse, err := http.Get(server.URL + "/basement/.well-known/oauth-protected-resource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	data, _ := io.ReadAll(httpResponse.Body)
	assert.JSONEq(t, `{"resource":"http://example.com/basement","authorization_servers":["oauth2"],"bearer_methods_supported":["header"]}`, string(data))
}

func TestDutchNutsProfile_identifiersFromCredential(t *testing.T) {
	uraCredential, err := vc.ParseVerifiableCredential(`{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://nuts.nl/credentials/2024",
    "https://w3c-ccg.github.io/lds-jws2020/contexts/lds-jws2020-v1.json"
  ],
  "credentialSubject": {
    "id": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea",
    "organization": {
      "city": "Utrecht",
      "name": "Demo Clinic",
      "ura": "1234"
    }
  },
  "id": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea#685f2ddf-fb1f-4618-bdcd-c2c16dd24646",
  "issuanceDate": "2024-07-17T11:30:48.441027305Z",
  "issuer": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea",
  "proof": {
    "created": "2024-07-17T11:30:48.441027305Z",
    "jws": "eyJhbGciOiJFUzI1NiIsImI2NCI6ZmFsc2UsImNyaXQiOlsiYjY0Il0sImtpZCI6ImRpZDp3ZWI6aDV0Y2dieHotNzA4MC5ldXcuZGV2dHVubmVscy5tczppYW06ZGQ2NDdkYjYtMWZkMS00ZWExLTk2OTEtNzUwNDFmM2Q2NmVhIzAifQ..E3XI1uSdMjBr4-7FRn5QZdhUvy92KY9PRlH-ia_x8V5Isvg7Ol0BXFlv60DQbWszLg_TdhE91jfSz_BOPHWgqQ",
    "proofPurpose": "assertionMethod",
    "type": "JsonWebSignature2020",
    "verificationMethod": "did:web:h5tcgbxz-7080.euw.devtunnels.ms:iam:dd647db6-1fd1-4ea1-9691-75041f3d66ea#0"
  },
  "type": [
    "NutsUraCredential",
    "VerifiableCredential"
  ]
}`)
	require.NoError(t, err)

	profile := &DutchNutsProfile{}
	identifiers, err := profile.identifiersFromCredential(*uraCredential)

	require.NoError(t, err)
	assert.Len(t, identifiers, 1)
	assert.Equal(t, coolfhir.URANamingSystem, *identifiers[0].System)
	assert.Equal(t, "1234", *identifiers[0].Value)
}
