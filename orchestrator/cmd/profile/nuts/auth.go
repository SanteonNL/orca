package nuts

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Authenticator authenticates the caller according to the Nuts authentication.
// The caller is required to provide an OAuth2 access token in the Authorization header, which is introspected at the Nuts node.
// The result is then mapped to a FHIR Organization resource, using Dutch coding systems (URA).
func (d DutchNutsProfile) Authenticator(fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request) {
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

				organization, err := claimsToOrganization(userInfo)
				if err != nil {
					slog.ErrorContext(request.Context(), "Invalid user info in context", slog.String("error", err.Error()))
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
