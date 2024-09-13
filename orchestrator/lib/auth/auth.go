package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
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
	return context.WithValue(ctx, principalContextKey, principal)
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
