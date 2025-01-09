package profile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
)

var _ Provider = TestProfile{}

func Test() TestProfile {
	return TestProfile{
		Principal: auth.TestPrincipal1,
		CSD: TestCsdDirectory{
			Endpoint: "https://example.com/fhir",
		},
	}
}

// TestProfile is a Profile implementation for testing purposes, that does very basic authentication that asserts the right HTTP Client is used.
type TestProfile struct {
	Principal              *auth.Principal
	xSCPContextHeaderValue string
	CSD                    csd.Directory
}

func (t TestProfile) Identities(_ context.Context) ([]fhir.Organization, error) {
	return []fhir.Organization{t.Principal.Organization}, nil
}

func (t TestProfile) CsdDirectory() csd.Directory {
	return t.CSD
}

func (t TestProfile) Authenticator(_ *url.URL, fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		_, err := auth.PrincipalFromContext(request.Context())
		if err == nil {
			fn(writer, request)
			return
		}
		authHeader := request.Header.Get("Authorization")
		if authHeader == "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Parse auth header
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		accessToken := authHeader[len("Bearer "):]
		data, err := base64.StdEncoding.DecodeString(accessToken)
		if err != nil {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(err.Error()))
			return
		}
		var user fhir.Organization
		err = json.Unmarshal(data, &user)
		if err != nil {
			writer.WriteHeader(http.StatusUnauthorized)
			_, _ = writer.Write([]byte(err.Error()))
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
		Transport: auth.AuthenticatedTestRoundTripper(nil, principal, t.xSCPContextHeaderValue),
	}
}

var _ csd.Directory = TestCsdDirectory{}

type TestCsdDirectory struct {
	Endpoint string
}

func (t TestCsdDirectory) LookupEntity(ctx context.Context, identifier fhir.Identifier) (*fhir.Reference, error) {
	return nil, csd.ErrEntryNotFound
}

func (t TestCsdDirectory) LookupEndpoint(_ context.Context, owner *fhir.Identifier, endpointName string) ([]fhir.Endpoint, error) {
	return []fhir.Endpoint{
		{
			Address: t.Endpoint,
		},
	}, nil
}
