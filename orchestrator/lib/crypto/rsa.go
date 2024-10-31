package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"hash"
)

var _ Suite = &RsaSuite{}

type RsaSuite struct {
	PrivateKey *rsa.PrivateKey
	Cert       *x509.Certificate
}

func (t RsaSuite) Certificate() *x509.Certificate {
	return t.Cert
}

func (t RsaSuite) SigningKey() crypto.Signer {
	return t.PrivateKey
}

func (t RsaSuite) DecryptRsaOaep(cipherText []byte, dm DigestMethod) ([]byte, error) {
	var h hash.Hash
	switch dm {
	case DigestMethodSha1:
		h = sha1.New()
	case DigestMethodSha256:
		h = sha256.New()
	default:
		return nil, errors.New("unsupported DigestMethod")
	}
	return rsa.DecryptOAEP(h, rand.Reader, t.PrivateKey, cipherText, nil)
}
