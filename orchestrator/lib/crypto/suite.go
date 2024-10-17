package crypto

import (
	"crypto"
)

type DigestMethod string

const DigestMethodSha1 = "http://www.w3.org/2000/09/xmldsig#sha1"
const DigestMethodSha256 = "http://www.w3.org/2000/09/xmldsig#sha256"

type Suite interface {
	SigningKey() crypto.Signer
	DecryptRsaOaep(cipherText []byte, digestMethod DigestMethod) ([]byte, error)
}
