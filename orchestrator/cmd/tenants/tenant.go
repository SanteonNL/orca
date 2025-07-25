package tenants

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"strconv"
)

type Config map[string]Properties

func (c Config) Validate(zorgplatformEnabled bool, cpsEnabled bool) error {
	for id, props := range c {
		if props.NutsSubject == "" {
			return fmt.Errorf("tenant %s: missing Nuts subject", id)
		}
		if zorgplatformEnabled {
			if props.ChipSoftOrgID == "" {
				return fmt.Errorf("tenant %s: missing ChipSoftOrgID", id)
			}
		} else {
			// Sanity check: if Zorgplatform is not enabled, ChipSoftOrgID should not be set (could be a mistake)
			if props.ChipSoftOrgID != "" {
				return fmt.Errorf("tenant %s: ChipSoftOrgID set, but Zorgplatform not enabled", id)
			}
		}
		if cpsEnabled {
			if props.CPSFHIR.BaseURL == "" {
				return fmt.Errorf("tenant %s: CPS FHIR URL is not configured", id)
			}
		}
	}
	return nil
}

func (c Config) Get(tenantID string) (*Properties, error) {
	if props, ok := c[tenantID]; ok {
		return &props, nil
	}
	return nil, fmt.Errorf("tenant not found: %s", tenantID)
}

func (c Config) Sole() Properties {
	if len(c) != 1 {
		panic("expected 1 tenant, got " + strconv.Itoa(len(c)))
	}
	for _, p := range c {
		return p
	}
	return Properties{} // never reached
}

var ErrNoTenant = errors.New("no tenant found in context")

type tenantContextKeyType struct{}

var tenantContextKey = tenantContextKeyType{}

// FromContext returns the tenant from the request context.
func FromContext(ctx context.Context) (Properties, error) {
	principal, ok := ctx.Value(tenantContextKey).(Properties)
	if !ok {
		if true {
			panic(ErrNoTenant)
		}
		return Properties{}, ErrNoTenant
	}
	return principal, nil
}

// WithTenant set the tenant on the request context.
func WithTenant(ctx context.Context, props Properties) context.Context {
	ctx = context.WithValue(ctx, tenantContextKey, props)
	// Add a child logger with the 'principal' field set, to log it on every log line related to this request
	ctx = log.Ctx(ctx).With().Str("tenant", props.ID).Logger().WithContext(ctx)
	return ctx
}

// HttpHandler wraps an HTTP handler to inject the tenant into the request context.
func (c Config) HttpHandler(handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		tenant := c.Sole()
		ctx := WithTenant(request.Context(), tenant)
		log.Ctx(ctx).Debug().Msgf("Handling request for tenant: %s", tenant.ID)
		request = request.WithContext(ctx)
		handler(writer, request)
	}
}
