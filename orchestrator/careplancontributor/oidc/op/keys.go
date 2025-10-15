package op

import (
	"crypto"
	"github.com/go-jose/go-jose/v4"
	"github.com/zitadel/oidc/v3/pkg/op"
)

var _ op.Key = (*PublicKey)(nil)

type PublicKey struct {
	id           string
	key          crypto.PublicKey
	sigAlgorithm jose.SignatureAlgorithm
}

func (p PublicKey) ID() string {
	return p.id
}

func (p PublicKey) Algorithm() jose.SignatureAlgorithm {
	return p.sigAlgorithm
}

func (p PublicKey) Use() string {
	return "sig"
}

func (p PublicKey) Key() any {
	return p.key
}

var _ op.SigningKey = (*SigningKey)(nil)

type SigningKey struct {
	id           string
	sigAlgorithm jose.SignatureAlgorithm
	key          crypto.Signer
}

func (p SigningKey) Public() PublicKey {
	return PublicKey{
		id:           p.id,
		key:          p.key.Public(),
		sigAlgorithm: p.sigAlgorithm,
	}
}

func (s SigningKey) SignatureAlgorithm() jose.SignatureAlgorithm {
	return s.sigAlgorithm
}

func (s SigningKey) Key() any {
	return s.key
}

func (s SigningKey) ID() string {
	return s.id
}
