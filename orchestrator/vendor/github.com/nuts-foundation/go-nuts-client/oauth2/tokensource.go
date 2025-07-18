package oauth2

import (
	"net/http"
	"net/url"
	"time"
)

type Token struct {
	AccessToken string
	TokenType   string
	Expiry      *time.Time
}

type TokenSource interface {
	Token(httpRequest *http.Request, authzServerURL *url.URL, scope string, noCache bool) (*Token, error)
}
