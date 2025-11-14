package nuts

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/SanteonNL/nuts-policy-enforcement-point/middleware"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = baseotel.Tracer("nuts")

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
		ctx, span := tracer.Start(
			request.Context(),
			debug.GetFullCallerName(),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()
		request = request.WithContext(ctx)

		principal, err := auth.PrincipalFromContext(request.Context())
		if err != nil {
			span.SetAttributes(attribute.String(otel.AuthNMethod, otel.AuthNMethodNuts))
			middleware.Secure(authConfig, func(response http.ResponseWriter, request *http.Request) {
				userInfo := middleware.UserInfo(request.Context())
				if userInfo == nil {
					// would be weird, should've been handled by middleware.Secure()
					respondAuthError(ctx, response, span, errors.New("user info not found in context"))
					return
				}
				subject, err := d.claimsToNutsSubject(userInfo)
				if err != nil {
					respondAuthError(ctx, response, span, errors.New("can't determine Nuts subject from token introspection response"))
					return
				}

				// If there's a tenant in the context, validate that it matches the Nuts subject.
				tenant, err := tenants.FromContext(request.Context())
				if err != nil && !errors.Is(err, tenants.ErrNoTenant) {
					respondAuthError(ctx, response, span, fmt.Errorf("getting tenant from context: %w", err))
					return
				} else if err == nil {
					if tenant.Nuts.Subject != subject {
						respondAuthError(ctx, response, span, fmt.Errorf("nuts access token issuer does not match tenant: expected %s, got %s", tenant.Nuts.Subject, subject))
						return
					}
				}

				organization, err := claimsToOrganization(userInfo)
				if err != nil {
					respondAuthError(ctx, response, span, fmt.Errorf("invalid user info in context: %w", err))
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
				span.SetAttributes(attribute.String(otel.AuthNOutcome, otel.AuthNOutcomeOK))
				fn(response, request.WithContext(auth.WithPrincipal(request.Context(), principal)))
			})(writer, request)
		} else {
			span.SetAttributes(attribute.String(otel.AuthNMethod, otel.AuthNMethodNuts+"(pre-authenticated)"))
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

func respondAuthError(ctx context.Context, response http.ResponseWriter, span trace.Span, err error) {
	span.SetAttributes(attribute.String(otel.AuthNOutcome, otel.AuthNOutcomeFailed))
	otel.Error(span, err)
	slog.ErrorContext(ctx, "Nuts authentication failed", slog.String(logging.FieldError, err.Error()))
	http.Error(response, "Unauthorized", http.StatusUnauthorized)
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
