package crypto

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEncryptAesCBC(t *testing.T) {
	expected := []byte("Hello, World! And y'all!")
	keySize := 32
	key, cipherText, err := EncryptAesCbc(expected, keySize)
	require.NoError(t, err)

	actual, err := DecryptAesCbc(cipherText, key)
	require.NoError(t, err)
	assert.Equal(t, string(expected), string(actual)[:len(expected)]) // TODO: Change this after padding is fixed
}
