package azkeyvault

import (
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"net/http"
	"time"
)

const AzureKeyVaultTimeout = 10 * time.Second

var AzureHttpRequestDoer HttpRequestDoer = http.DefaultClient

type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type SigningKey interface {
	crypto.Signer
	SigningAlgorithm() string
	KeyID() string
}

func GetKey(client *azkeys.Client, keyName string) (SigningKey, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

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

func NewClient(keyVaultURL string, insecure bool) (*azkeys.Client, error) {
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

	return azkeys.NewClient(keyVaultURL, cred, clientOptions) // never returns an error
}
