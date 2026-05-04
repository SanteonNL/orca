package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRsaSuite(t *testing.T) RsaSuite {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return RsaSuite{PrivateKey: key, Cert: &x509.Certificate{}}
}

func TestRsaSuite_Certificate(t *testing.T) {
	suite := newTestRsaSuite(t)
	assert.Equal(t, suite.Cert, suite.Certificate())
}

func TestRsaSuite_SigningKey(t *testing.T) {
	suite := newTestRsaSuite(t)
	assert.Equal(t, suite.PrivateKey, suite.SigningKey())
}

func TestRsaSuite_DecryptRsaOaep(t *testing.T) {
	suite := newTestRsaSuite(t)
	plaintext := []byte("hello world")

	t.Run("SHA1", func(t *testing.T) {
		ciphertext, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, &suite.PrivateKey.PublicKey, plaintext, nil)
		require.NoError(t, err)
		result, err := suite.DecryptRsaOaep(ciphertext, DigestMethodSha1)
		require.NoError(t, err)
		assert.Equal(t, plaintext, result)
	})
	t.Run("SHA256", func(t *testing.T) {
		ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &suite.PrivateKey.PublicKey, plaintext, nil)
		require.NoError(t, err)
		result, err := suite.DecryptRsaOaep(ciphertext, DigestMethodSha256)
		require.NoError(t, err)
		assert.Equal(t, plaintext, result)
	})
	t.Run("unsupported digest method", func(t *testing.T) {
		_, err := suite.DecryptRsaOaep([]byte("data"), DigestMethod("unsupported"))
		require.EqualError(t, err, "unsupported DigestMethod")
	})
}
