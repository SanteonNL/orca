package crypto

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateNonce creates a 256-bit secure random
func GenerateNonce() string {
	buf := make([]byte, 256/8)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}
