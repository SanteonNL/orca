package azkeyvault

import (
	"context"
	"crypto"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"io"
)

// SigningKeyFromAzureKeyVault reads a key from Azure KeyVault and returns it as SigningKey.
// It must be an Elliptic Curve key.
func SigningKeyFromAzureKeyVault(keyVaultURL, keyName string) (SigningKey, error) {
	client, err := NewClient(keyVaultURL, false)
	if err != nil {
		return nil, err
	}
	return GetKey(client, keyName)
}

func Sign(digest []byte, keyVaultURL string, keyName string, keyVersion string) (azkeys.SignResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	client, err := NewClient(keyVaultURL, false)
	if err != nil {
		return azkeys.SignResponse{}, fmt.Errorf("unable to acquire Azure client: %w", err)
	}

	return client.Sign(ctx, keyName, keyVersion, azkeys.SignParameters{
		Algorithm: to.Ptr(azkeys.SignatureAlgorithmRS256),
		Value:     digest,
	}, nil)
}

var _ SigningKey = &azureSigningKey{}

type azureSigningKey struct {
	keyName   string
	key       jwk.Key
	publicKey crypto.PublicKey
	client    *azkeys.Client
}

func (a azureSigningKey) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	// Sanity check
	if opts != nil && opts.HashFunc() == 0 {
		return nil, errors.New("hashing should've been done")
	}

	response, err := a.client.Sign(ctx, a.keyName, "", azkeys.SignParameters{
		Algorithm: to.Ptr(azkeys.SignatureAlgorithm(a.SigningAlgorithm())),
		Value:     digest,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to sign with Azure KeyVault: %w", err)
	}
	return response.Result, nil
}

func (a azureSigningKey) Public() crypto.PublicKey {
	return a.publicKey
}

func (a azureSigningKey) SigningAlgorithm() string {
	return a.key.Algorithm().String()
}

func (a azureSigningKey) KeyID() string {
	return a.key.KeyID()
}

func setKeyAlg(parsedKey jwk.Key, key *azkeys.JSONWebKey) error {
	switch parsedKey.KeyType() {
	case jwa.EC:
		switch *key.Crv {
		case azkeys.CurveNameP256:
			return parsedKey.Set(jwk.AlgorithmKey, jwa.ES256)
		case azkeys.CurveNameP256K:
			return parsedKey.Set(jwk.AlgorithmKey, jwa.ES256K)
		case azkeys.CurveNameP384:
			return parsedKey.Set(jwk.AlgorithmKey, jwa.ES384)
		case azkeys.CurveNameP521:
			return parsedKey.Set(jwk.AlgorithmKey, jwa.ES512)
		default:
			return fmt.Errorf("unsupported curve: %s", *key.Crv)
		}
	default:
		return fmt.Errorf("unsupported key type: %s", parsedKey.KeyType())
	}
}
