package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

type principalContextKeyType struct{}

var principalContextKey = principalContextKeyType{}

// Middleware returns a http.HandlerFunc that retrieves the user info from the request header.
// The user info is to be provided by an API Gateway/reverse proxy that validated the authentication token in the request.
// It sets the decoded user info in the request context.
func Middleware(authConfig middleware.Config, fn http.HandlerFunc) func(writer http.ResponseWriter, request *http.Request) {
	return middleware.Secure(authConfig, func(response http.ResponseWriter, request *http.Request) {
		userInfo := middleware.UserInfo(request.Context())
		if userInfo == nil {
			// would be weird, should've been handled by middleware.Secure()
			log.Error().Msg("User info not found in context")
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}

		organization, err := claimsToOrganization(userInfo)
		if err != nil {
			log.Error().Err(err).Msg("Invalid user info in context")
			http.Error(response, "Unauthorized", http.StatusUnauthorized)
			return
		}
		principal := Principal{
			Organization: *organization,
		}
		log.Info().Msgf("Authenticated user: %v", principal)
		newCtx := context.WithValue(request.Context(), principalContextKey, principal)
		fn(response, request.WithContext(newCtx))
	})
}

func claimsToOrganization(claims map[string]interface{}) (*fhir.Organization, error) {
	ura, ok := claims["organization_ura"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_ura claim in user info")
	}
	name, ok := claims["organization_name"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_name claim in user info")
	}
	city, ok := claims["organization_city"].(string)
	if !ok || ura == "" {
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

var _ fmt.Stringer = Principal{}

type Principal struct {
	Organization fhir.Organization
}

func (u Principal) String() string {
	return fmt.Sprintf("Organization (%s=%s, name=%s, city=%s)",
		*u.Organization.Identifier[0].System,
		*u.Organization.Identifier[0].Value,
		*u.Organization.Name,
		*u.Organization.Address[0].City)
}
