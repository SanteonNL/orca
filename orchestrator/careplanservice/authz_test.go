package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/stretchr/testify/assert"
	"testing"
)

type AuthzPolicyTest[T any] struct {
	name       string
	principal  *auth.Principal
	wantAllow  bool
	wantErr    error
	skipReason string
	resource   T
	policy     Policy[T]
}

func testPolicies[T any](t *testing.T, tests []AuthzPolicyTest[T]) {
	t.Helper()
	for _, tt := range tests {
		testPolicy(t, tt)
	}
}

func testPolicy[T any](t *testing.T, tt AuthzPolicyTest[T]) {
	t.Helper()
	t.Run(tt.name, func(t *testing.T) {
		if tt.skipReason != "" {
			t.Skip(tt.skipReason)
		}
		hasAccess, err := tt.policy.HasAccess(context.Background(), tt.resource, *tt.principal)
		assert.Equal(t, tt.wantAllow, hasAccess.Allowed)
		if tt.wantErr != nil {
			assert.EqualError(t, err, tt.wantErr.Error())
		} else {
			assert.NoError(t, err)
		}
	})
}
