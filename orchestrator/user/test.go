package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func SessionFromHttpResponse[T any](manager *SessionManager[T], httpResponse *http.Response) *T {
	// extract session ID; sid=<something>;
	cookieValue := httpResponse.Header.Get("Set-Cookie")
	cookieValue = strings.Split(cookieValue, ";")[0]
	cookieValue = strings.Split(cookieValue, "=")[1]

	httpRequest := httptest.NewRequest("GET", "/", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: cookieValue,
	})
	return manager.Get(httpRequest)
}
