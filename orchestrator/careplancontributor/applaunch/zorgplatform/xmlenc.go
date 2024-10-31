package zorgplatform

import (
	"encoding/base64"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/crypto"
	"github.com/beevik/etree"
	"github.com/braineet/saml/xmlenc"
	"strings"
)

func init() {
	xmlenc.RegisterDecrypter(RsaOaepXmlSuite{})
}

var _ xmlenc.Decrypter = &RsaOaepXmlSuite{}

// RsaOaepXmlSuite is a xmlenc.Decrypter that can decrypt using RSA-OAEP-MGF1P,
// with a potentially external key.
type RsaOaepXmlSuite struct {
}

func (e RsaOaepXmlSuite) Algorithm() string {
	return "http://www.w3.org/2001/04/xmlenc#rsa-oaep-mgf1p"
}

func (e RsaOaepXmlSuite) Decrypt(key interface{}, ciphertextEl *etree.Element) ([]byte, error) {
	castKey, ok := key.(crypto.Suite)
	if !ok {
		return nil, fmt.Errorf("expected key to be a crypto.Suite")
	}
	ciphertext, err := getCiphertext(ciphertextEl)
	if err != nil {
		return nil, err
	}
	digestMethodEl := ciphertextEl.FindElement("./EncryptionMethod/DigestMethod")
	return castKey.DecryptRsaOaep(ciphertext, crypto.DigestMethod(digestMethodEl.SelectAttrValue("Algorithm", "")))
}

func getCiphertext(encryptedKey *etree.Element) ([]byte, error) {
	ciphertextEl := encryptedKey.FindElement("./CipherData/CipherValue")
	if ciphertextEl == nil {
		return nil, fmt.Errorf("cannot find CipherData element containing a CipherValue element")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(strings.TrimSpace(ciphertextEl.Text()))
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}
