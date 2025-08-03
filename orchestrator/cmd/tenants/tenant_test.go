package tenants

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("Nuts configuration", func(t *testing.T) {
		t.Run("subject not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID: "sub",
				},
			}
			err := c.Validate(false)
			require.EqualError(t, err, "tenant sub: missing Nuts subject")
		})
	})
	t.Run("CarePlanService configuration", func(t *testing.T) {
		t.Run("CPSFHIR BaseURL not set", func(t *testing.T) {
			c := Config{
				"sub": Properties{
					ID: "sub",
					Nuts: NutsProperties{
						Subject: "subject",
					},
				},
			}
			err := c.Validate(true)
			require.EqualError(t, err, "tenant sub: CPS FHIR URL is not configured")
		})
	})
}

func Test_isIDValid(t *testing.T) {
	type args struct {
		tenantID string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "valid ID",
			args: args{tenantID: "valid-id_123"},
			want: true,
		},
		{
			name: "empty ID",
			args: args{tenantID: ""},
			want: false,
		},
		{
			name: "invalid characters",
			args: args{tenantID: "invalid@id!"},
			want: false,
		},
		{
			name: "only alphanumeric, dashes, and underscores",
			args: args{tenantID: "valid-id_123-456"},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIDValid(tt.args.tenantID); got != tt.want {
				t.Errorf("isIDValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctx := WithTenant(context.Background(), Properties{
			ID: "test-tenant",
			Nuts: NutsProperties{
				Subject: "test-subject",
			},
		})
		props, err := FromContext(ctx)
		require.NoError(t, err)
		require.Equal(t, "test-tenant", props.ID)
		require.Equal(t, "test-subject", props.Nuts.Subject)
	})
	t.Run("no tenant in context", func(t *testing.T) {
		ctx := context.Background()
		_, err := FromContext(ctx)
		require.ErrorIs(t, err, ErrNoTenant)
	})
}
