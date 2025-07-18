package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func main() {
	println("Generating key pair...")
	privateKeyJWK, publicKeyJWKS := generate()
	println("----------------------")
	println("Private key:")
	println("----------------------")
	println(privateKeyJWK)
	println("----------------------")
	println("Public key:")
	println("----------------------")
	println(publicKeyJWKS)
	println()
	println()

	println("Checking key pair...")
	if err := testKeyPair(privateKeyJWK, publicKeyJWKS); err != nil {
		panic(err)
	}
	println("Key pair OK")
}

func testKeyPair(privateKeyJWK, publicJWKSJSON string) error {
	// Test by loading key, then signing JWT and checking with the JWT set
	publicKeySet, err := jwk.Parse([]byte(publicJWKSJSON))
	if err != nil {
		return err
	}
	privateKey, err := jwk.ParseKey([]byte(privateKeyJWK))
	if err != nil {
		return err
	}

	token := jwt.New()
	if err := token.Set(jwt.IssuerKey, "https://example.com"); err != nil {
		return err
	}
	if err := token.Set(jwt.SubjectKey, "test"); err != nil {
		return err
	}

	jwtHeaders := jws.NewHeaders()
	if err := jwtHeaders.Set(jwk.KeyIDKey, privateKey.KeyID()); err != nil {
		return err
	}
	jwtBytes, err := jwt.Sign(token, jwt.WithKey(jwa.ES384, privateKey))
	if err != nil {
		return err
	}

	// Now check signature with JWK set
	_, err = jwt.Parse(jwtBytes, jwt.WithKeySet(publicKeySet))
	if err != nil {
		return err
	}
	return nil
}

// generate generates a new private key and returns the private key as JWK and the public key as JWK set.
func generate() (string, string) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		panic(err)
	}
	privSet := jwk.NewSet()
	privateKeyAsJWK, err := jwk.FromRaw(privateKey)
	if err != nil {
		panic(err)
	}
	if err := privateKeyAsJWK.Set(jwk.AlgorithmKey, jwa.ES384); err != nil {
		panic(err)
	}
	if err := privateKeyAsJWK.Set(jwk.KeyIDKey, uuid.NewString()); err != nil {
		panic(err)
	}
	if err := privSet.AddKey(privateKeyAsJWK); err != nil {
		panic(err)
	}
	publicSet, err := jwk.PublicSetOf(privSet)
	if err != nil {
		panic(err)
	}
	publicJWSSBytes, err := json.MarshalIndent(publicSet, "", "  ")
	if err != nil {
		panic(err)
	}
	privateJWKBytes, err := json.MarshalIndent(privateKeyAsJWK, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(privateJWKBytes), string(publicJWSSBytes)
}
