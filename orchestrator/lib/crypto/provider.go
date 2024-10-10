package crypto

import (
	"crypto"
)

type Suite interface {
	SigningKey() crypto.Signer
	DecryptRsaOaep(cipherText []byte) ([]byte, error)
}
