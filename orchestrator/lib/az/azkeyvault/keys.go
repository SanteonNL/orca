package azkeyvault

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azkeys"
	libCrypto "github.com/SanteonNL/orca/orchestrator/lib/crypto"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/rs/zerolog/log"
)

const AzureKeyVaultTimeout = 10 * time.Second

var AzureHttpRequestDoer HttpRequestDoer = http.DefaultClient

type HttpRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

var _ libCrypto.Suite = &Suite{}

type Suite struct {
	keyPair
}

type keyPair struct {
	keyName                 string
	keyVersion              string
	asJwk                   jwk.Key
	publicKey               crypto.PublicKey
	client                  KeysClient
	publicKeyThumbprintS256 []byte
}

func (s Suite) PublicKeyThumbprintS256() []byte {
	return s.publicKeyThumbprintS256
}

func (s Suite) SigningKey() crypto.Signer {
	return s
}

func (s Suite) DecryptRsaOaep(cipherText []byte, dm libCrypto.DigestMethod) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	var alg azkeys.EncryptionAlgorithm
	switch dm {
	case libCrypto.DigestMethodSha1:
		alg = azkeys.EncryptionAlgorithmRSAOAEP
	case libCrypto.DigestMethodSha256:
		alg = azkeys.EncryptionAlgorithmRSAOAEP256
	default:
		return nil, fmt.Errorf("unsupported DigestMethod: %s", dm)
	}

	decryptResponse, err := s.client.Decrypt(ctx, s.keyName, s.keyVersion, azkeys.KeyOperationParameters{
		Algorithm: to.Ptr(alg),
		Value:     cipherText,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt with Azure KeyVault: %w", err)
	}
	return decryptResponse.Result, err
}

func GetKey(client KeysClient, keyName string) (*Suite, error) {
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

	thumbprintS256, err := publicKeyJWK.Thumbprint(crypto.SHA256)
	if err != nil {
		return nil, err
	}

	var publicKey crypto.PublicKey
	if err = publicKeyJWK.Raw(&publicKey); err != nil {
		return nil, fmt.Errorf("unable to parse public key from Azure KeyVault: %w", err)
	}
	return &Suite{
		keyPair{
			keyName:                 key.KID.Name(),
			keyVersion:              key.KID.Version(),
			asJwk:                   parsedKey,
			publicKey:               publicKey,
			publicKeyThumbprintS256: thumbprintS256,
			client:                  client,
		},
	}, nil
}

func createCredential(credentialType string) (azcore.TokenCredential, error) {
	switch credentialType {
	case "default":
		return azidentity.NewDefaultAzureCredential(nil)
	case "cli":
		return azidentity.NewAzureCLICredential(nil)
	case "managed_identity":
		opts := &azidentity.ManagedIdentityCredentialOptions{
			ClientOptions: azcore.ClientOptions{},
		}
		// For UserAssignedManagedIdentity, client ID needs to be explicitly set.
		// Taken from github.com/!azure/azure-sdk-for-go/sdk/azidentity@v1.7.0/default_azure_credential.go:100
		if ID, ok := os.LookupEnv("AZURE_CLIENT_ID"); ok {
			log.Logger.Debug().Msg("Azure: configuring UserAssignedManagedIdentity (using AZURE_CLIENT_ID) for Azure Key Vault client.")
			opts.ID = azidentity.ClientID(ID)
		}
		return azidentity.NewManagedIdentityCredential(opts)
	default:
		return nil, fmt.Errorf("unsupported Azure Key Vault credential type: %s", credentialType)
	}
}

func NewKeysClient(keyVaultURL string, credentialType string, insecure bool) (*azkeys.Client, error) {
	cred, err := createCredential(credentialType)
	if err != nil {
		return nil, fmt.Errorf("unable to acquire Azure credential: %w", err)
	}
	var clientOptions *azkeys.ClientOptions
	if insecure {
		clientOptions = &azkeys.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				InsecureAllowCredentialWithHTTP: true,
			},
		}
	} else {
		clientOptions = &azkeys.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: AzureHttpRequestDoer,
			},
		}
	}
	return azkeys.NewClient(keyVaultURL, cred, clientOptions) // never returns an error
}

func (a keyPair) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), AzureKeyVaultTimeout)
	defer cancel()

	// Sanity check
	if opts != nil && opts.HashFunc() == 0 {
		return nil, errors.New("hashing should've been done")
	}
	var signingAlgorithm azkeys.SignatureAlgorithm
	if pssOpts, ok := opts.(*rsa.PSSOptions); ok {
		switch pssOpts.Hash.Size() {
		case sha256.Size:
			signingAlgorithm = azkeys.SignatureAlgorithmPS256
		default:
			return nil, fmt.Errorf("unsupported PSS hash size: %d", pssOpts.Hash.Size())
		}
	} else {
		switch opts.HashFunc().Size() {
		case sha256.Size:
			signingAlgorithm = azkeys.SignatureAlgorithmRS256
		default:
			return nil, fmt.Errorf("unsupported RSA hash size: %d", pssOpts.Hash.Size())
		}
	}
	response, err := a.client.Sign(ctx, a.keyName, a.keyVersion, azkeys.SignParameters{
		Algorithm: to.Ptr(signingAlgorithm),
		Value:     digest,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to sign with Azure KeyVault: %w", err)
	}
	return response.Result, nil
}

func (a keyPair) Public() crypto.PublicKey {
	return a.publicKey
}

func (a keyPair) SigningAlgorithm() string {
	return a.asJwk.Algorithm().String()
}

func (a keyPair) KeyID() string {
	return a.asJwk.KeyID()
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
	case jwa.RSA:
		return parsedKey.Set(jwk.AlgorithmKey, jwa.RS256)
	default:
		return fmt.Errorf("unsupported key type: %s", parsedKey.KeyType())
	}
}
