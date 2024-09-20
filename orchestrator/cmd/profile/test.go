package profile

import (
	"encoding/base64"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
)

var _ Provider = TestProfile{}

// TestProfile is a Profile implementation for testing purposes, that does very basic authentication that asserts the right HTTP Client is used.
type TestProfile struct {
	Principal *auth.Principal
}

func (t TestProfile) CsdDirectory() csd.Directory {
	panic("implement me")
}

func (t TestProfile) Authenticator(_ *url.URL, fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		authHeader := request.Header.Get("Authorization")
		if authHeader == "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Parse auth header
		if !strings.HasPrefix(authHeader, "Bearer ") {
			println("Authentication header does not start with 'Bearer '")
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		accessToken := authHeader[len("Bearer "):]
		data, err := base64.StdEncoding.DecodeString(accessToken)
		if err != nil {
			println(err)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		var user fhir.Organization
		err = json.Unmarshal(data, &user)
		if err != nil {
			println(err)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		request = request.WithContext(auth.WithPrincipal(request.Context(), auth.Principal{
			Organization: user,
		}))
		fn(writer, request)
	}
}

func (t TestProfile) RegisterHTTPHandlers(_ string, _ *url.URL, _ *http.ServeMux) {

}

func (t TestProfile) HttpClient() *http.Client {
	var principal *auth.Principal
	if t.Principal == nil {
		principal = auth.TestPrincipal1
	} else {
		principal = t.Principal
	}
	return &http.Client{
		Transport: auth.AuthenticatedTestRoundTripper(nil, principal),
	}
}