package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"hash"
)

var _ Suite = &RsaSuite{}

type RsaSuite struct {
	PrivateKey *rsa.PrivateKey
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
