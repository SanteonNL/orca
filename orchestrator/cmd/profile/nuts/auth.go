package nuts

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Authenticator authenticates the caller according to the Nuts authentication.
// The caller is required to provide an OAuth2 access token in the Authorization header, which is introspected at the Nuts node.
// The result is then mapped to a FHIR Organization resource, using Dutch coding systems (URA).
//
// Example token introspection response:
//
//	{
//	 "active": true,
//	 "client_id": "http://localhost:8080/oauth2/hospital",
//	 "exp": 1759941958,
//	 "iat": 1759941058,
//	 "iss": "http://localhost:8080/oauth2/clinic",
//	 "organization_city": "Amsterdam",
//	 "organization_name": "Demo Hospital",
//	 "organization_ura": "4567",
//	 "scope": "careplanservice"
//	}
func (d DutchNutsProfile) Authenticator(fn http.HandlerFunc) http.HandlerFunc {
	authConfig := middleware.Config{
		TokenIntrospectionEndpoint: d.Config.API.Parse().JoinPath("internal/auth/v2/accesstoken/introspect").String(),
		TokenIntrospectionClient:   d.nutsAPIHTTPClient,
	}
	return func(writer http.ResponseWriter, request *http.Request) {
		principal, err := auth.PrincipalFromContext(request.Context())
		if err != nil {
			middleware.Secure(authConfig, func(response http.ResponseWriter, request *http.Request) {
				userInfo := middleware.UserInfo(request.Context())
				if userInfo == nil {
					// would be weird, should've been handled by middleware.Secure()
					slog.ErrorContext(request.Context(), "User info not found in context")
					http.Error(response, "Unauthorized", http.StatusUnauthorized)
					return
				}
				subject, err := d.claimsToNutsSubject(userInfo)
				if err != nil {
					slog.ErrorContext(request.Context(), "Can't determine Nuts subject from token introspection response", slog.String(logging.FieldError, err.Error()))
					http.Error(response, "Unauthorized", http.StatusUnauthorized)
					return
				}

				// If there's a tenant in the context, validate that it matches the Nuts subject.
				tenant, err := tenants.FromContext(request.Context())
				if err != nil && !errors.Is(err, tenants.ErrNoTenant) {
					slog.ErrorContext(request.Context(), "Error getting tenant from context", slog.String(logging.FieldError, err.Error()))
					http.Error(response, "Unauthorized", http.StatusUnauthorized)
					return
				} else if err == nil {
					if tenant.Nuts.Subject != subject {
						slog.ErrorContext(request.Context(), "Nuts access token issuer does not match tenant", slog.String("expected", tenant.Nuts.Subject), slog.String("got", subject))
						http.Error(response, "Unauthorized", http.StatusUnauthorized)
						return
					}
				}

				organization, err := claimsToOrganization(userInfo)
				if err != nil {
					slog.ErrorContext(request.Context(), "Invalid user info in context", slog.String(logging.FieldError, err.Error()))
					http.Error(response, "Unauthorized", http.StatusUnauthorized)
					return
				}
				principal := auth.Principal{
					Organization: *organization,
				}
				slog.DebugContext(
					request.Context(),
					"Authenticated user",
					slog.Any("principal", principal),
					slog.String("route", request.URL.Path),
				)
				fn(response, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
			})(writer, request)
		} else {
			slog.DebugContext(
				request.Context(),
				"Pre-authenticated user",
				slog.Any("principal", principal),
				slog.String("route", request.URL.Path),
			)
			fn(writer, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
		}
	}
}

func (d DutchNutsProfile) claimsToNutsSubject(userInfo map[string]interface{}) (string, error) {
	issuer, ok := userInfo["iss"].(string)
	if !ok || issuer == "" {
		return "", errors.New("missing iss claim in user info")
	}
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return "", errors.New("invalid iss claim in user info")
	}
	// Last path part = Nuts subject
	pathParts := strings.Split(issuerURL.Path, "/")
	result := pathParts[len(pathParts)-1]
	if result == "" {
		return "", errors.New("invalid issuer; can't determine Nuts subject")
	}
	return result, nil
}

func claimsToOrganization(claims map[string]interface{}) (*fhir.Organization, error) {
	ura, ok := claims["organization_ura"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_ura claim in user info")
	}
	name, ok := claims["organization_name"].(string)
	if !ok || name == "" {
		return nil, errors.New("missing organization_name claim in user info")
	}
	city, ok := claims["organization_city"].(string)
	if !ok || city == "" {
		return nil, errors.New("missing organization_city claim in user info")
	}

	return &fhir.Organization{
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr(coolfhir.URANamingSystem),
				Value:  to.Ptr(ura),
			},
		},
		Name: to.Ptr(name),
		Address: []fhir.Address{
			{
				City: to.Ptr(city),
			},
		},
	}, nil
}
