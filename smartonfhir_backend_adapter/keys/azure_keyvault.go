package keys

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"io"
	"net/http"
	"time"
)

const AzureKeyVaultTimeout = 10 * time.Second

type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var AzureHttpRequestDoer HttpRequestDoer = http.DefaultClient

// SigningKeyFromAzureKeyVault reads a key from Azure KeyVault and returns it as SigningKey.
// It must be an Elliptic Curve key.
func SigningKeyFromAzureKeyVault(keyVaultURL, keyName string) (SigningKey, error) {
	return signingKeyFromAzureKeyVault(keyVaultURL, keyName, false)
}

func signingKeyFromAzureKeyVault(keyVaultURL, keyName string, insecure bool) (SigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()
	var cred *azidentity.DefaultAzureCredential
	var clientOptions *azkeys.ClientOptions
	var err error
	if insecure {
		clientOptions = &azkeys.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				InsecureAllowCredentialWithHTTP: true,
				Transport:                       AzureHttpRequestDoer,
			},
		}
	} else {
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("unable to acquire Azure credential: %w", err)
		}
	}
	client, _ := azkeys.NewClient(keyVaultURL, cred, clientOptions) // never returns an error
	keyResponse, err := client.GetKey(ctx, keyName, "", nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get key from Azure KeyVault: %w", err)
	}
	key := keyResponse.Key
	// Parse into jwk.Key
	jwkBytes, _ := json.Marshal(key)
	parsedKey, err := jwk.ParseKey(jwkBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse key from Azure KeyVault: %w", err)
	}
	// Find out SigningAlgorithm
	if err := setKeyAlg(parsedKey, key); err != nil {
		return nil, fmt.Errorf("unable to set JWK alg: %w", err)
	}
	// Pre-parse the PublicKey
	publicKeyJWK, err := parsedKey.PublicKey()
	if err != nil {
		return nil, fmt.Errorf("unable to parse public key from Azure KeyVault: %w", err)
	}
	var publicKey crypto.PublicKey
	if err = publicKeyJWK.Raw(&publicKey); err != nil {
		return nil, fmt.Errorf("unable to parse public key from Azure KeyVault: %w", err)
	}
	return &azureSigningKey{
		keyName:   keyName,
		key:       parsedKey,
		publicKey: publicKey,
		client:    client,
	}, nil
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
