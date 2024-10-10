package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/beevik/etree"
	"strings"
)

/*
File taken form https://github.com/aykevl/go-xmlenc/blob/master/xmlenc.go
*/

import (
	"encoding/base64"
	"errors"
)

func DecryptXmlEnc(src *etree.Element, decryptor Decryptor) (*etree.Document, error) {
	// Do some preliminary checks.
	// Warning: these are very specific and should be expanded for more support,
	// at some point.
	if src.Tag != "EncryptedData" {
		return nil, errors.New("xmlenc: element to decrypt must be EncryptedData, not " + src.Tag)
	}
	if src.SelectAttrValue("Type", "") != "http://www.w3.org/2001/04/xmlenc#Element" {
		return nil, errors.New("xmlenc: EncryptedData is not an element but " + src.SelectAttrValue("Type", ""))
	}
	encMethod := src.FindElement("EncryptionMethod").SelectAttrValue("Algorithm", "")
	if encMethod != "http://www.w3.org/2001/04/xmlenc#aes256-cbc" {
		return nil, errors.New("xmlenc: unsupported symmetric key algorithm: " + encMethod)
	}
	pubkeyAlgo := src.FindElement("KeyInfo/EncryptedKey/EncryptionMethod").SelectAttrValue("Algorithm", "")
	if pubkeyAlgo != "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p" {
		return nil, errors.New("xmlenc: unsupported public key algorithm: " + pubkeyAlgo)
	}
	digestMethod := src.FindElement("KeyInfo/EncryptedKey/EncryptionMethod/DigestMethod").SelectAttrValue("Algorithm", "")
	if digestMethod != "http://www.w3.org/2000/09/xmldsig#sha1" {
		return nil, errors.New("xmlenc: unsupported digest method: " + digestMethod)
	}

	// Decode the ciphertext from base64.

	pubkeyCiphertextB64 := strings.TrimSpace(src.FindElement("KeyInfo/EncryptedKey/CipherData/CipherValue").Text())
	pubkeyCiphertext, err := base64.StdEncoding.DecodeString(pubkeyCiphertextB64)
	if err != nil {
		return nil, err
	}

	ciphertextB64 := strings.TrimSpace(src.FindElement("CipherData/CipherValue").Text())
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}

	// Decrypt the key (which is encrypted using RSA).
	symmetricKey, err := decryptor(pubkeyCiphertext)
	if err != nil {
		return nil, fmt.Errorf("xmlenc: error decrypting symmetric key: %w", err)
	}
	block, err := aes.NewCipher(symmetricKey)
	if err != nil {
		return nil, err
	}

	// Decrypt the ciphertext with AES-CBC.
	// Note: this is not secure against padding oracles! The ciphertext MUST be
	// verified before decryption!
	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(data, data)

	// Strip the padding.
	paddingLen := data[len(data)-1]
	if paddingLen >= aes.BlockSize {
		return nil, errors.New("xmlenc: invalid padding")
	}
	data = data[:len(data)-int(paddingLen)]

	// Parse the resulting element.
	doc := etree.NewDocument()
	err = doc.ReadFromBytes(data)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

type Decryptor func(ciphertext []byte) ([]byte, error)
