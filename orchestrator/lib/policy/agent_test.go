package policy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type input map[string]any

func TestAllow(t *testing.T) {
	tests := map[string]struct {
		input         input
		policy        string
		expectedError error
	}{
		"request is allowed": {
			input:  input{"allow": true},
			policy: "allow := input.allow",
		},
		"request is not allowed": {
			input:         input{"allow": false},
			policy:        "allow := input.allow",
			expectedError: ErrAccessDenied,
		},
		"evaluation failed": {
			input:         input{},
			policy:        "",
			expectedError: errors.New("policy evaluation failed"),
		},
		"invalid type for allow": {
			input:         input{},
			policy:        "allow := 10",
			expectedError: errors.New("invalid type for allow"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.TODO()
			agent, err := NewAgent(ctx, RegoModule{
				Package: "example",
				Source:  fmt.Sprintf("package example\n%s", tt.policy),
			})
			require.NoError(t, err)

			request, err := http.NewRequest("GET", "http://localhost", nil)
			require.NoError(t, err)

			err = agent.Allow(ctx, tt.input, request)

			if tt.expectedError != nil {
				require.Equal(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
