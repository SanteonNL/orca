package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
)

func NewTestSuite() *TestSuite {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	return &TestSuite{
		PrivateKey: privateKey,
	}
}

var _ Suite = &TestSuite{}

type TestSuite struct {
	PrivateKey *rsa.PrivateKey
}

func (t TestSuite) SigningKey() crypto.Signer {
	return t.PrivateKey
}

func (t TestSuite) DecryptRsaOaep(cipherText []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, t.PrivateKey, cipherText, nil)
}

func (t TestSuite) EncryptRsaOaep(plainText []byte, label []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, t.PrivateKey.Public().(*rsa.PublicKey), plainText, label)
}
