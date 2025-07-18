package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"strings"
)

type userInfoContextKeyType struct{}

var userInfoContextKey = userInfoContextKeyType{}

// Secure wraps the given handler in middleware that checks the access token in the request headers.
// If the access token is valid, the handler is called. If the access token is invalid, a 401 Unauthorized response is returned.
func Secure(config Config, handler func(response http.ResponseWriter, request *http.Request)) func(response http.ResponseWriter, request *http.Request) {
	return func(response http.ResponseWriter, request *http.Request) {
		_, accessToken, err := parseAuthorizationHeader(request.Header.Get("Authorization"))
		if err != nil {
			log.Ctx(request.Context()).Info().Msgf("Invalid Authorization header: %s", err)
			respondUnauthorized(config, "invalid_request", err.Error(), response)
			return
		}
		introspectionResponse, err := IntrospectAccessToken(accessToken, config.TokenIntrospectionEndpoint, config.TokenIntrospectionClient)
		if err != nil {
			log.Error().Err(err).Msg("Failed to introspect access token")
			respondUnauthorized(config, "server_error", "couldn't verify access token", response)
			return
		}
		if !introspectionResponse.Active() {
			log.Ctx(request.Context()).Info().Msg("Access token is unknown or expired")
			respondUnauthorized(config, "invalid_request", "invalid/expired token", response)
			return
		}
		userInfo := *introspectionResponse
		newCtx := context.WithValue(request.Context(), userInfoContextKey, map[string]interface{}(userInfo))
		handler(response, request.WithContext(newCtx))
	}
}

// UserInfo returns the user information that was resulted from the introspection of the access token.
// If the user information is not available (meaning Token Introspection wasn't performed), nil is returned.
func UserInfo(ctx context.Context) map[string]interface{} {
	userInfo, _ := ctx.Value(userInfoContextKey).(map[string]interface{})
	return userInfo
}

type IntrospectionResult map[string]interface{}

func (i IntrospectionResult) Active() bool {
	b, ok := i["active"].(bool)
	return ok && b
}

func IntrospectAccessToken(accessToken string, endpoint string, httpClient *http.Client) (*IntrospectionResult, error) {
	body := url.Values{
		"token": {accessToken},
	}
	httpResponse, err := httpClient.PostForm(endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, fmt.Errorf("http status: %d", httpResponse.StatusCode)
	}
	var result IntrospectionResult
	if err := json.NewDecoder(httpResponse.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("json decode error: %w", err)
	}
	return &result, nil
}

func respondUnauthorized(config Config, errorCode string, errorDescription string, response http.ResponseWriter) {
	// escape errorCode and errorDescription
	errorCode = strings.ReplaceAll(errorCode, `"`, `\"`)
	errorDescription = strings.ReplaceAll(errorDescription, `"`, `\"`)
	errorDescription = strings.ReplaceAll(errorDescription, `"`, `\"`)
	wwwAuthParams := []string{
		fmt.Sprintf(`error="%s"`, errorCode),
		fmt.Sprintf(`error_description="%s"`, errorDescription),
	}
	if config.BaseURL != nil {
		resourceMetadataURL := config.BaseURL.JoinPath(".well-known", "oauth-protected-resource")
		wwwAuthParams = append(wwwAuthParams, fmt.Sprintf(`resource_metadata="%s"`, resourceMetadataURL))
	}
	response.Header().Set("WWW-Authenticate", fmt.Sprintf("Bearer %s", strings.Join(wwwAuthParams, ", ")))
	response.WriteHeader(http.StatusUnauthorized)
}

func parseAuthorizationHeader(authorizationHeader string) (string, string, error) {
	if authorizationHeader == "" {
		return "", "", fmt.Errorf("missing Authorization header")
	}
	parts := strings.Split(authorizationHeader, " ")
	if len(parts) < 2 {
		return "", "", fmt.Errorf("invalid Authorization header")
	}
	if parts[0] != "Bearer" {
		return "", "", fmt.Errorf("unsupported token type")
	}
	return parts[0], parts[1], nil
}
