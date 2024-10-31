package user

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func SessionFromHttpResponse(manager *SessionManager, httpResponse *http.Response) *SessionData {
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
