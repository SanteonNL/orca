package smart_on_fhir

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/SanteonNL/orca/smartonfhir_backend_adapter/keys"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBackendTokenSource_createGrant(t *testing.T) {
	privateKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	signingKey, err := jwk.FromRaw(privateKey)
	require.NoError(t, err)
	require.NoError(t, signingKey.Set(jwk.KeyIDKey, "test"))
	require.NoError(t, signingKey.Set(jwk.AlgorithmKey, jwa.ES256))
	tokenSource := BackendTokenSource{
		OAuth2ASTokenEndpoint: "https://example.com/as",
		ClientID:              "123456",
		SigningKey: keys.JWKSigningKey{
			JWK: signingKey,
		},
	}

	grant, err := tokenSource.createGrant()
	require.NoError(t, err)
	t.Log(grant)
}
