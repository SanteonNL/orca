package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

type userInfoContextKeyType struct{}

var userInfoContextKey = userInfoContextKeyType{}

// Middleware returns a http.HandlerFunc that retrieves the user info from the request header.
// The user info is to be provided by an API Gateway/reverse proxy that validated the authentication token in the request.
// It sets the decoded user info in the request context.
func Middleware(fn http.HandlerFunc) func(writer http.ResponseWriter, request *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		userInfo, err := parseUserInfo(request)
		if err != nil {
			log.Info().Msgf("Auth middleware: %v", err)
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		log.Info().Msgf("Authenticated user: %v", userInfo)
		newCtx := context.WithValue(request.Context(), userInfoContextKey, userInfo)
		request = request.WithContext(newCtx)
		fn(writer, request)
	}
}

func parseUserInfo(request *http.Request) (*UserInfo, error) {
	header := request.Header.Get("X-Userinfo")
	if len(header) == 0 {
		return nil, errors.New("missing X-Userinfo request header")
	}
	userinfoJSON, err := base64.StdEncoding.DecodeString(header)
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode X-Userinfo header: %w", err)
	}
	claims := make(map[string]interface{})
	if err := json.Unmarshal(userinfoJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to JSON unmarshal X-Userinfo header: %w", err)
	}

	organization, err := claimsToOrganization(claims)
	if err != nil {
		return nil, err
	}
	return &UserInfo{
		Organization: *organization,
	}, nil
}

func claimsToOrganization(claims map[string]interface{}) (*fhir.Organization, error) {
	ura, ok := claims["organization_ura"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_ura claim in X-Userinfo header")
	}
	name, ok := claims["organization_name"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_name claim in X-Userinfo header")
	}
	city, ok := claims["organization_city"].(string)
	if !ok || ura == "" {
		return nil, errors.New("missing organization_city claim in X-Userinfo header")
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

var _ fmt.Stringer = UserInfo{}

type UserInfo struct {
	Organization fhir.Organization
}

func (u UserInfo) String() string {
	return fmt.Sprintf("Organization (%s=%s, name=%s, city=%s)",
		*u.Organization.Identifier[0].System,
		*u.Organization.Identifier[0].Value,
		*u.Organization.Name,
		*u.Organization.Address[0].City)
}
