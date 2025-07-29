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

func (c Config) Validate(cpsEnabled bool) error {
	for id, props := range c {
		if props.Nuts.Subject == "" {
			return fmt.Errorf("tenant %s: missing Nuts subject", id)
		}
		if cpsEnabled {
			if props.CPS.FHIR.BaseURL == "" {
				return fmt.Errorf("tenant %s: CPS FHIR URL is not configured", id)
			}
		}
	}
	return nil
}

func isIDValid(tenantID string) bool {
	// Only alphanumeric, dashes, and underscores are allowed in tenant IDs
	for _, r := range tenantID {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_') {
			return false
		}
	}
	return len(tenantID) > 0
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
		tenantID := request.PathValue("tenant")
		if tenantID == "" {
			log.Ctx(request.Context()).Error().Msgf("No tenant ID in HTTP request path: %s", request.URL.Path)
			http.Error(writer, "Unable to determine tenant", http.StatusInternalServerError)
			return
		}
		tenant, err := c.Get(tenantID)
		if err != nil {
			log.Ctx(request.Context()).Error().Err(err).Msgf("Unknown tenant (id=%s, request=%s)", tenantID, request.URL.Path)
			http.Error(writer, "Unknown tenant", http.StatusBadRequest)
			return
		}
		ctx := WithTenant(request.Context(), *tenant)
		log.Ctx(ctx).Debug().Msgf("Handling request for tenant: %s", tenant.ID)
		request = request.WithContext(ctx)
		handler(writer, request)
	}
}
