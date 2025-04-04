package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type principalContextKeyType struct{}

var principalContextKey = principalContextKeyType{}

var ErrNotAuthenticated = errors.New("not authenticated")

// PrincipalFromContext returns the principal from the request context.
func PrincipalFromContext(ctx context.Context) (Principal, error) {
	principal, ok := ctx.Value(principalContextKey).(Principal)
	if !ok {
		return Principal{}, ErrNotAuthenticated
	}
	return principal, nil
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	ctx = context.WithValue(ctx, principalContextKey, principal)
	// Add a child logger with the 'principal' field set, to log it on every log line related to this request
	ctx = log.Ctx(ctx).With().Str("principal", principal.ID()).Logger().WithContext(ctx)
	return ctx
}

var _ fmt.Stringer = Principal{}

type Principal struct {
	Organization fhir.Organization
}

func (u Principal) ID() string {
	return coolfhir.ToString(u.Organization.Identifier[0])
}

func (u Principal) String() string {
	return fmt.Sprintf("Organization (%s=%s, name=%s, city=%s)",
		*u.Organization.Identifier[0].System,
		*u.Organization.Identifier[0].Value,
		*u.Organization.Name,
		*u.Organization.Address[0].City)
}
