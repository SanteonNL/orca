package keys

import (
	"crypto"
	"errors"
	"fmt"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"io"
	"os"
)

var _ SigningKey = &JWKSigningKey{}

type JWKSigningKey struct {
	JWK jwk.Key
}

func (j JWKSigningKey) Public() crypto.PublicKey {
	var pubKey crypto.PublicKey
	if err := j.JWK.Raw(&pubKey); err != nil {
		return nil
	}
	return pubKey
}

func (j JWKSigningKey) Sign(rand io.Reader, digest []byte, opts crypto.SignerOpts) (signature []byte, err error) {
	var signer crypto.Signer
	if err := j.JWK.Raw(&signer); err != nil {
		return nil, fmt.Errorf("unable to create signer from JWK: %w", err)
	}
	return signer.Sign(rand, digest, opts)
}

func (j JWKSigningKey) SigningAlgorithm() string {
	return j.JWK.Algorithm().String()
}

func (j JWKSigningKey) KeyID() string {
	return j.JWK.KeyID()
}

// SigningKeyFromJWKFile reads a JWK file and returns a SigningKey
func SigningKeyFromJWKFile(jwkFile string) (SigningKey, error) {
	data, err := os.ReadFile(jwkFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read JWK file: %w", err)
	}
	jwkKey, err := jwk.ParseKey(data)
	if err != nil {
		return nil, fmt.Errorf("invalid JWK file: %w", err)
	}
	if jwkKey.KeyID() == "" {
		return nil, errors.New("JWK file does not contain a key ID")
	}
	if jwkKey.Algorithm().String() == "" {
		return nil, errors.New("JWK file does not contain a signing algorithm")
	}

	return &JWKSigningKey{JWK: jwkKey}, nil
}
