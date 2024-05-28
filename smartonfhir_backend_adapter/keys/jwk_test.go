package keys

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSigningKeyFromJWKFile(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		key, err := SigningKeyFromJWKFile("../test/private.jwk")
		require.NoError(t, err)

		assert.Equal(t, "7c87bdbc-ef89-41a7-a940-70d7e7b1a828", key.KeyID())
		assert.Equal(t, "ES384", key.SigningAlgorithm())
		assert.NotNil(t, key.Public())
	})
	t.Run("no key ID", func(t *testing.T) {
		_, err := SigningKeyFromJWKFile("../test/private_no_keyid.jwk")
		require.EqualError(t, err, "JWK file does not contain a key ID")
	})
	t.Run("no signing algorithm", func(t *testing.T) {
		_, err := SigningKeyFromJWKFile("../test/private_no_alg.jwk")
		require.EqualError(t, err, "JWK file does not contain a signing algorithm")
	})
}
