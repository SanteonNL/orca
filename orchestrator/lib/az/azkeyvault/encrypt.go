package azkeyvault

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
)

func Decrypt(digest []byte, keyVaultURL string, keyName string, keyVersion string) (azkeys.DecryptResponse, error) {
	return decryptWithKeyVault(digest, keyVaultURL, keyName, keyVersion, false)
}

func Encrypt(client *azkeys.Client, digest []byte, keyName string, keyVersion string) (azkeys.EncryptResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	return client.Encrypt(ctx, keyName, keyVersion, azkeys.KeyOperationParameters{
		Algorithm: to.Ptr(azkeys.EncryptionAlgorithmRSAOAEP),
		Value:     digest,
	}, nil)
}

//TODO: Dynamically retrieve this from the encrypted saml token
// func getKeyAlg(alg string) (azkeys.EncryptionAlgorithm, error) {

// 	switch alg {
// 	case "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p":
// 		return azkeys.EncryptionAlgorithmRSAOAEP, nil

// 	return azkeys.EncryptionAlgorithm(""), fmt.Errorf("unsupported encryption algorithm: %s", alg)
// }

func decryptWithKeyVault(digest []byte, keyVaultURL string, keyName string, keyVersion string, insecure bool) (azkeys.DecryptResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	client, err := NewClient(keyVaultURL, insecure)
	if err != nil {
		return azkeys.DecryptResponse{}, fmt.Errorf("unable to acquire Azure client: %w", err)
	}

	// foundAlg, err := getKeyAlg(keyAlg)

	return client.Decrypt(ctx, keyName, keyVersion, azkeys.KeyOperationParameters{
		Algorithm: to.Ptr(azkeys.EncryptionAlgorithmRSAOAEP),
		Value:     digest,
	}, nil)
}
