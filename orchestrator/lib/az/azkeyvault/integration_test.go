package azkeyvault

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"os"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/crypto"
	"github.com/stretchr/testify/require"
)

func Test_Suite_DecryptRsaOaep(t *testing.T) {
	t.Skip()
	// Configure these values in order to run this test:
	const vaultUrl = "https://testkvint.vault.azure.net/"
	const keyName = "testkey"
	// You can use several ways to authenticate, see https://blog.nashtechglobal.com/how-to-authenticate-azure-using-golang/
	const credentialType = "default" // could also be managed_identity
	os.Setenv("AZURE_CLIENT_ID", "")
	os.Setenv("AZURE_USERNAME", "")
	os.Setenv("AZURE_PASSWORD", "")

	plainText := []byte("Hello, World!")
	client, err := NewKeysClient(vaultUrl, credentialType, false)
	require.NoError(t, err)
	suite, err := GetKey(client, keyName)
	require.NoError(t, err)
	publicKey := suite.SigningKey().Public().(*rsa.PublicKey)

	// Encrypt in Go
	cipherText, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, plainText, nil)
	require.NoError(t, err)

	// Decrypt in Azure Key Vault
	decrypted, err := suite.DecryptRsaOaep(cipherText, crypto.DigestMethodSha256)
	require.NoError(t, err)
	require.Equal(t, plainText, decrypted)
}
