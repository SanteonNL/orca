package tenants

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

type Config map[string]Properties

func (c Config) Validate(cpsEnabled bool) error {
	for id, props := range c {
		if !isIDValid(id) {
			return fmt.Errorf("tenant %s: invalid ID", id)
		}
		if props.Nuts.Subject == "" {
			return fmt.Errorf("tenant %s: missing Nuts subject", id)
		}
		if cpsEnabled {
			if props.CPS.FHIR.BaseURL == "" {
				return fmt.Errorf("tenant %s: CPS FHIR URL is not configured", id)
			}
			if err := props.CPS.FHIR.Validate(); err != nil {
				return fmt.Errorf("tenant %s: invalid CPS FHIR configuration: %w", id, err)
			}
		}
		if err := props.Demo.FHIR.Validate(); err != nil {
			return fmt.Errorf("tenant %s: invalid Demo FHIR configuration: %w", id, err)
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
		return Properties{}, ErrNoTenant
	}
	return principal, nil
}

// WithTenant set the tenant on the request context.
func WithTenant(ctx context.Context, props Properties) context.Context {
	ctx = context.WithValue(ctx, tenantContextKey, props)
	// Add the tenant to the context for logging
	ctx = logging.AppendCtx(ctx, slog.String("tenant", props.ID))
	return ctx
}

// HttpHandler wraps an HTTP handler to inject the tenant into the request context.
func (c Config) HttpHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		tenantID := request.PathValue("tenant")
		if tenantID == "" {
			slog.ErrorContext(request.Context(), "No tenant ID in HTTP request", slog.String(logging.FieldPath, request.URL.Path))
			http.Error(writer, "Tenant is required in request path", http.StatusBadRequest)
			return
		}
		tenant, err := c.Get(tenantID)
		if err != nil {
			slog.ErrorContext(request.Context(), "Unknown tenant", slog.String(logging.FieldError, err.Error()),
				slog.String(logging.FieldPath, request.URL.Path),
				slog.String("tenant_id", tenantID),
			)
			http.Error(writer, "Invalid tenant provided in path", http.StatusBadRequest)
			return
		}
		ctx := WithTenant(request.Context(), *tenant)
		slog.DebugContext(ctx, "Handling request", slog.String("tenant_id", tenant.ID))
		request = request.WithContext(ctx)
		handler(writer, request)
	}
}
