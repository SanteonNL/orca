package smart_on_fhir

import (
	"crypto"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jws"
	"io"
	"net/http"
	"net/url"
	"time"
)

// grantTokenValidity specifies how long the grant token (used to acquire the access token) is valid.
const grantTokenValidity = 5 * time.Second

var _ oauth2.TokenSource = &BackendTokenSource{}

// BackendTokenSource is an oauth2.TokenSource for a SMART on FHIR backend client.
type BackendTokenSource struct {
	OAuth2ASTokenEndpoint string
	ClientID              string
	SigningKey            jwk.Key
}

func (p BackendTokenSource) Token() (*oauth2.Token, error) {
	log.Debug().Msg("Refreshing OAuth2 Access Token")
	grantJWT, err := p.createGrant()
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT grant: %w", err)
	}
	token, err := p.exchange(grantJWT)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve token: %w", err)
	}
	return token, nil
}

func (p BackendTokenSource) exchange(grantJWT string) (*oauth2.Token, error) {
	// Loosely inspired by golang.org/x/oauth2@v0.19.0/jwt/jwt.go
	// Specified by https://fhir.epic.com/Documentation?docId=oauth2&section=Backend-Oauth2_Getting-Access-Token
	v := url.Values{}
	v.Set("grant_type", "client_credentials")
	v.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	v.Set("client_assertion", grantJWT)
	// TODO: Epic on FHIR SMART OAuth2 Backend service ignores this, but SMART on FHIR Sandbox requires it?
	v.Set("scope", "system/*.cruds")
	response, err := http.PostForm(p.OAuth2ASTokenEndpoint, v)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch token: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024)) // 1mb
	if err != nil {
		return nil, fmt.Errorf("cannot fetch token: %v", err)
	}
	if c := response.StatusCode; c < 200 || c > 299 {
		return nil, &oauth2.RetrieveError{
			Response: response,
			Body:     body,
		}
	}
	return parseTokenResponse(body)
}

func (p BackendTokenSource) createGrant() (string, error) {
	// Create JWT according to https://fhir.epic.com/Documentation?docId=oauth2&section=Creating-JWTs
	// For Epic:
	//claims := map[string]interface{}{
	//	jwt.IssuerKey:     p.ClientID,
	//	jwt.SubjectKey:    p.ClientID,
	//	jwt.AudienceKey:   p.OAuth2ASTokenEndpoint,
	//	jwt.JwtIDKey:      uuid.NewString(),
	//	jwt.ExpirationKey: time.Now().Add(grantTokenValidity),
	//	jwt.IssuedAtKey:   time.Now(),
	//	jwt.NotBeforeKey:  time.Now(),
	//}
	//tokenBuilder := jwt.New()
	//for claimName, claimValue := range claims {
	//	if err := tokenBuilder.Set(claimName, claimValue); err != nil {
	//		return "", fmt.Errorf("invalid JWT claim %s: %w", claimName, err)
	//	}
	//}
	//signedToken, err := jwt.Sign(tokenBuilder, jwt.WithKey(p.SigningKey.Algorithm(), p.SigningKey))
	//if err != nil {
	//	return "", fmt.Errorf("failed to sign JWT: %w", err)
	//}
	//return string(signedToken), nil

	// For SMART on FHIR Sandbox (does not support audience as array):
	hdr := &jws.Header{
		Algorithm: p.SigningKey.Algorithm().String(),
		Typ:       "JWT",
		KeyID:     p.SigningKey.KeyID(),
	}
	claims := &jws.ClaimSet{
		Iss: p.ClientID,
		Aud: p.OAuth2ASTokenEndpoint,
		Exp: time.Now().Add(grantTokenValidity).Unix(),
		Iat: time.Now().Unix(),
		Sub: p.ClientID,
		PrivateClaims: map[string]interface{}{
			"jti": uuid.NewString(),
			"nbf": time.Now().Unix(),
		},
	}
	var signer crypto.Signer
	if err := p.SigningKey.Raw(&signer); err != nil {
		return "", fmt.Errorf("failed to get private key: %w", err)
	}
	return jws.EncodeWithSigner(hdr, claims, func(data []byte) ([]byte, error) {
		return signer.Sign(rand.Reader, data, nil)
	})
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	Scope       string `json:"scope"`
}

func parseTokenResponse(data []byte) (*oauth2.Token, error) {
	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: tr.AccessToken,
		TokenType:   tr.TokenType,
		Expiry:      time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second).Add(-10 * time.Second), // allow for some clock skew
	}, nil
}
