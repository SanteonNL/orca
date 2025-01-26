package nuts

import (
	"errors"
	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

var tokenIntrospectionClient = http.DefaultClient

// Authenticator authenticates the caller according to the Nuts authentication.
// The caller is required to provide an OAuth2 access token in the Authorization header, which is introspected at the Nuts node.
// The result is then mapped to a FHIR Organization resource, using Dutch coding systems (URA).
func (d DutchNutsProfile) Authenticator(resourceServerURL *url.URL, fn func(writer http.ResponseWriter, request *http.Request)) func(writer http.ResponseWriter, request *http.Request) {
	authConfig := middleware.Config{
		TokenIntrospectionEndpoint: d.Config.API.Parse().JoinPath("internal/auth/v2/accesstoken/introspect").String(),
		TokenIntrospectionClient:   tokenIntrospectionClient,
		BaseURL:                    resourceServerURL,
	}
	return middleware.Secure(authConfig, func(response http.ResponseWriter, request *http.Request) {
		userInfo := middleware.UserInfo(request.Context())
		if userInfo == nil {
			// would be weird, should've been handled by middleware.Secure()
			log.Ctx(request.Context()).Error().Msg("User info not found in context")
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		organization, err := claimsToOrganization(userInfo)
		if err != nil {
			log.Ctx(request.Context()).Err(err).Msg("Invalid user info in context")
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}
		principal := auth.Principal{
			Organization: *organization,
		}
		log.Ctx(request.Context()).Debug().Msgf("Authenticated user: %v", principal)
		fn(response, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
	})
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
