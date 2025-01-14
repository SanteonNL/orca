package ehr

import (
	"context"
	"encoding/base64"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"time"
)
import "encoding/json"
import "strings"

type AzureOauthClient interface {
	GetAzureCredential() (*azidentity.DefaultAzureCredential, error)
	GetBearerToken(spt *azidentity.DefaultAzureCredential) (*OAuthBearerToken, error)
}

type AzureOauthClientImpl struct {
}

func NewAzureOauthClient() (AzureOauthClient, error) {
	return &AzureOauthClientImpl{}, nil
}

// Claims is the main container for our body information
type Claims map[string]interface{}

type OAuthBearerToken struct {
	TokenValue string
	Expiration time.Time
	Principal  string
	Extensions map[string]string
}

func getClaimsFromJwt(tokenStr string) (Claims, error) {
	tokenArray := strings.Split(tokenStr, ".")

	claimsByte, err := base64.RawURLEncoding.DecodeString(tokenArray[1])
	if err != nil {
		return nil, err
	}

	var claims Claims
	err = json.Unmarshal(claimsByte, &claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func getExpirationFromClaims(claims Claims) time.Time {
	if obj, ok := claims["exp"]; ok {
		if expVal, ok := obj.(float64); ok {
			return time.Unix(int64(expVal), 0)
		}
	}

	return time.Now()
}

func (k *AzureOauthClientImpl) GetBearerToken(spt *azidentity.DefaultAzureCredential) (*OAuthBearerToken, error) {
	extensions := map[string]string{}

	// Acquire a new token and extract expiry
	token, err := spt.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"https://evhns-pro-tst-we-001-cklyhpuklcxy2.servicebus.windows.net"},
	})
	if err != nil {
		return nil, err
	}

	tokenString := token.Token

	oauthBearerToken := OAuthBearerToken{
		TokenValue: tokenString,
		Expiration: token.ExpiresOn,
		Principal:  "",
		Extensions: extensions,
	}

	return &oauthBearerToken, nil
}

func (k *AzureOauthClientImpl) GetAzureCredential() (*azidentity.DefaultAzureCredential, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return cred, nil
}
