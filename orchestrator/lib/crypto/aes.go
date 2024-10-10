package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

// EncryptAesCbc encrypts the given plaintext using AES in CBC mode. It prefixes the IV to the resulting ciphertext.
// It generates a random key of the given size and uses it to encrypt the plaintext.
// It returns the generated key, and the concatenation of the IV and the ciphertext.
func EncryptAesCbc(plainText []byte, keySize int) ([]byte, []byte, error) {
	if err := validateAesKeySize(keySize); err != nil {
		return nil, nil, err
	}
	if len(plainText) < aes.BlockSize {
		return nil, nil, fmt.Errorf("plaintext length (%d) must be at least the block size (%d)", len(plainText), aes.BlockSize)
	}
	generatedKey, err := GenerateKey(keySize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate AES key: %w", err)
	}
	block, err := aes.NewCipher(generatedKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create new AES cipher: %w", err)
	}
	// iv = blocksize = 16
	iv, err := GenerateKey(block.BlockSize())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate IV: %w", err)
	}
	paddedPlainText := PadPkcs5(plainText, block.BlockSize())
	cipherText := make([]byte, len(paddedPlainText))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText, paddedPlainText)
	result := append(iv, cipherText...)
	return generatedKey, result, nil
}

// DecryptAesCbc decrypts the given ciphertext using AES in CBC mode.
// It expects the IV to be the first block of the ciphertext.
// It uses the given key to decrypt the ciphertext.
// It returns the resulting plaintext.
func DecryptAesCbc(cipherText, key []byte) ([]byte, error) {
	if err := validateAesKeySize(len(key)); err != nil {
		return nil, err
	}
	if len(cipherText)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length (%d) must be a multiple of the block size (%d)", len(cipherText), aes.BlockSize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create new AES cipher: %w", err)
	}
	iv := cipherText[:block.BlockSize()]
	cipherText = cipherText[block.BlockSize():]
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(cipherText, cipherText)
	// TODO: Correct padding
	return cipherText, nil
}

func GenerateKey(size int) ([]byte, error) {
	symmetricKey := make([]byte, size)
	_, err := rand.Read(symmetricKey)
	if err != nil {
		return nil, err
	}
	return symmetricKey, nil
}

func validateAesKeySize(keySize int) error {
	if keySize != 16 && keySize != 24 && keySize != 32 {
		return fmt.Errorf("invalid key size %d", keySize)
	}
	return nil
}
