package auth

import (
	"context"
	"log/slog"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestPrincipalFromContext(t *testing.T) {
	tests := []struct {
		name        string
		ctx         context.Context
		setup       func(context.Context) context.Context
		expectedErr error
		expectedID  string
	}{
		{
			name:        "should return principal when present in context",
			ctx:         context.Background(),
			setup:       func(ctx context.Context) context.Context { return WithPrincipal(ctx, *TestPrincipal1) },
			expectedErr: nil,
			expectedID:  "http://fhir.nl/fhir/NamingSystem/ura|1",
		},
		{
			name:        "should return error when principal not in context",
			ctx:         context.Background(),
			setup:       func(ctx context.Context) context.Context { return ctx },
			expectedErr: ErrNotAuthenticated,
		},
		{
			name:        "should handle different principals",
			ctx:         context.Background(),
			setup:       func(ctx context.Context) context.Context { return WithPrincipal(ctx, *TestPrincipal2) },
			expectedErr: nil,
			expectedID:  "http://fhir.nl/fhir/NamingSystem/ura|2",
		},
		{
			name:        "should handle nil context",
			ctx:         nil,
			setup:       func(ctx context.Context) context.Context { return WithPrincipal(context.Background(), *TestPrincipal3) },
			expectedErr: nil,
			expectedID:  "http://fhir.nl/fhir/NamingSystem/ura|3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup(tt.ctx)
			principal, err := PrincipalFromContext(ctx)

			if tt.expectedErr != nil {
				assert.Equal(t, tt.expectedErr, err)
				assert.Equal(t, Principal{}, principal)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, principal.ID())
			}
		})
	}
}

func TestWithPrincipal(t *testing.T) {
	tests := []struct {
		name      string
		principal Principal
		expected  string
	}{
		{
			name:      "should add principal to context",
			principal: *TestPrincipal1,
			expected:  "Test Organization 1",
		},
		{
			name:      "should add different principal to context",
			principal: *TestPrincipal2,
			expected:  "Test Organization 2",
		},
		{
			name:      "should add third principal to context",
			principal: *TestPrincipal3,
			expected:  "Test Organization 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithPrincipal(context.Background(), tt.principal)

			// Verify the principal is in the context
			principal, err := PrincipalFromContext(ctx)
			assert.NoError(t, err)
			assert.Equal(t, *tt.principal.Organization.Name, *principal.Organization.Name)
		})
	}
}

func TestWithPrincipalAddsLoggingContext(t *testing.T) {
	t.Run("should add principal ID to logging context", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithPrincipal(ctx, *TestPrincipal1)

		// Verify logging context has been updated
		principal, err := PrincipalFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura|1", principal.ID())
	})
}

func TestPrincipalID(t *testing.T) {
	tests := []struct {
		name      string
		principal Principal
		expected  string
	}{
		{
			name:      "should return first identifier value",
			principal: *TestPrincipal1,
			expected:  "http://fhir.nl/fhir/NamingSystem/ura|1",
		},
		{
			name:      "should return correct ID for second principal",
			principal: *TestPrincipal2,
			expected:  "http://fhir.nl/fhir/NamingSystem/ura|2",
		},
		{
			name:      "should return correct ID for third principal",
			principal: *TestPrincipal3,
			expected:  "http://fhir.nl/fhir/NamingSystem/ura|3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := tt.principal.ID()
			assert.Equal(t, tt.expected, id)
		})
	}
}

func TestPrincipalString(t *testing.T) {
	tests := []struct {
		name             string
		principal        Principal
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name:      "should format principal correctly",
			principal: *TestPrincipal1,
			shouldContain: []string{
				"Organization",
				"Test Organization 1",
				"Bugland",
			},
		},
		{
			name:      "should include identifier information",
			principal: *TestPrincipal2,
			shouldContain: []string{
				"Test Organization 2",
				"Testland",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			str := tt.principal.String()
			for _, shouldContain := range tt.shouldContain {
				assert.Contains(t, str, shouldContain)
			}
			for _, shouldNotContain := range tt.shouldNotContain {
				assert.NotContains(t, str, shouldNotContain)
			}
		})
	}
}

func TestPrincipalStringer(t *testing.T) {
	t.Run("should implement fmt.Stringer", func(t *testing.T) {
		principal := TestPrincipal1
		str := principal.String()
		assert.NotEmpty(t, str)
		assert.IsType(t, "", str)
	})
}

func TestErrNotAuthenticated(t *testing.T) {
	t.Run("should have correct error message", func(t *testing.T) {
		assert.Equal(t, "not authenticated", ErrNotAuthenticated.Error())
	})
}

func TestPrincipalContextKeyIsolation(t *testing.T) {
	t.Run("should not leak principal between different context values", func(t *testing.T) {
		ctx1 := WithPrincipal(context.Background(), *TestPrincipal1)
		ctx2 := context.Background()

		principal1, err1 := PrincipalFromContext(ctx1)
		_, err2 := PrincipalFromContext(ctx2)

		assert.NoError(t, err1)
		assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura|1", principal1.ID())
		assert.Error(t, err2)
		assert.Equal(t, ErrNotAuthenticated, err2)
	})
}

func TestWithPrincipalMultipleCalls(t *testing.T) {
	t.Run("should overwrite principal when called multiple times", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithPrincipal(ctx, *TestPrincipal1)
		ctx = WithPrincipal(ctx, *TestPrincipal2)

		principal, err := PrincipalFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura|2", principal.ID())
	})
}

func TestWithPrincipalPreservesExistingContext(t *testing.T) {
	t.Run("should preserve other context values", func(t *testing.T) {
		type contextKey string
		const testKey contextKey = "test"
		const testValue = "test_value"

		ctx := context.Background()
		ctx = context.WithValue(ctx, testKey, testValue)
		ctx = WithPrincipal(ctx, *TestPrincipal1)

		// Verify both values are present
		assert.Equal(t, testValue, ctx.Value(testKey))
		principal, err := PrincipalFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura|1", principal.ID())
	})
}

func TestContextHandlerIntegration(t *testing.T) {
	t.Run("should work with logging context handler", func(t *testing.T) {
		ctx := context.Background()
		ctx = logging.AppendCtx(ctx, slog.String("test", "value"))
		ctx = WithPrincipal(ctx, *TestPrincipal1)

		principal, err := PrincipalFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura|1", principal.ID())
	})
}

func TestPrincipalWithCustomOrganization(t *testing.T) {
	t.Run("should handle custom organization", func(t *testing.T) {
		customOrg := fhir.Organization{
			Name: to.Ptr("Custom Org"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://example.com/org"),
					Value:  to.Ptr("custom-123"),
				},
			},
			Address: []fhir.Address{
				{
					City: to.Ptr("CustomCity"),
				},
			},
		}

		principal := Principal{Organization: customOrg}
		ctx := WithPrincipal(context.Background(), principal)

		retrieved, err := PrincipalFromContext(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "http://example.com/org|custom-123", retrieved.ID())
		assert.Equal(t, "Custom Org", *retrieved.Organization.Name)
	})
}
