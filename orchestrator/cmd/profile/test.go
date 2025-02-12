package profile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
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

func (t TestProfile) CapabilityStatement(cp *fhir.CapabilityStatement) {
	cp.Rest[0].Security = &fhir.CapabilityStatementRestSecurity{
		Service: []fhir.CodeableConcept{
			{
				Coding: []fhir.Coding{
					{
						System: to.Ptr("http://hl7.org/fhir/restful-security-service"),
						Code:   to.Ptr("SMART-on-FHIR"),
					},
				},
			},
		},
	}
}

func (t TestProfile) Identities(_ context.Context) ([]fhir.Organization, error) {
	return []fhir.Organization{t.Principal.Organization}, nil
}

func (t TestProfile) CsdDirectory() csd.Directory {
	return t.CSD
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

func (t TestProfile) HttpClient(_ context.Context, _ fhir.Identifier) (*http.Client, error) {
	p := t.Principal
	if p != nil {
		p = auth.TestPrincipal1
	}
	return &http.Client{
		Transport: auth.AuthenticatedTestRoundTripper(nil, p, t.xSCPContextHeaderValue),
	}, nil
}

var _ csd.Directory = TestCsdDirectory{}

type TestCsdDirectory struct {
	Endpoint  string
	Endpoints map[string]map[string]string
}

func (t TestCsdDirectory) LookupEntity(ctx context.Context, identifier fhir.Identifier) (*fhir.Reference, error) {
	return nil, csd.ErrEntryNotFound
}

func (t TestCsdDirectory) LookupEndpoint(_ context.Context, owner *fhir.Identifier, endpointName string) ([]fhir.Endpoint, error) {
	if t.Endpoints != nil {
		if owner == nil {
			// Return all endpoints of this type
			var endpoints []fhir.Endpoint
			for _, endpointsByOwner := range t.Endpoints {
				if endpoint, ok := endpointsByOwner[endpointName]; ok {
					endpoints = append(endpoints, fhir.Endpoint{
						Address: endpoint,
					})
				}
			}
			return endpoints, nil
		}
		if endpoints, ok := t.Endpoints[coolfhir.ToString(owner)]; ok {
			if endpoint, ok := endpoints[endpointName]; ok {
				return []fhir.Endpoint{
					{
						Address: endpoint,
					},
				}, nil
			}
		}
	}
	if t.Endpoint == "" {
		return nil, nil
	}
	return []fhir.Endpoint{
		{
			Address: t.Endpoint,
		},
	}, nil
}
