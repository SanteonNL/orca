package keys

import "crypto"

type SigningKey interface {
	crypto.Signer
	SigningAlgorithm() string
	KeyID() string
}
