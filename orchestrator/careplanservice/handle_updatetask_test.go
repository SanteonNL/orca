package careplanservice

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_isValidTransition(t *testing.T) {
	type args struct {
		from        fhir.TaskStatus
		to          fhir.TaskStatus
		isOwner     bool
		isRequester bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// Positive cases
		{
			name: "requested -> received : owner (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusReceived,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "requested -> accepted : owner (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusAccepted,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "requested -> rejected : owner (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusRejected,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : owner (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "requested -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "received -> accepted : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusAccepted,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "received -> rejected : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusRejected,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "received -> cancelled : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "received -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "received -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "accepted -> in-progress : owner (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusInProgress,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : owner (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : requester (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusCancelled,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "accepted -> cancelled : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusCancelled,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "in-progress -> completed : owner (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusCompleted,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "in-progress -> failed : owner (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusFailed,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : owner (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusOnHold,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : requester (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusOnHold,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "in-progress -> on-hold : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusOnHold,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : owner (OK)",
			args: args{
				from:        fhir.TaskStatusOnHold,
				to:          fhir.TaskStatusInProgress,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : requester (OK)",
			args: args{
				from:        fhir.TaskStatusOnHold,
				to:          fhir.TaskStatusInProgress,
				isOwner:     false,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "on-hold -> in-progress : owner & requester (OK)",
			args: args{
				from:        fhir.TaskStatusOnHold,
				to:          fhir.TaskStatusInProgress,
				isOwner:     true,
				isRequester: true,
			},
			want: true,
		},
		{
			name: "ready -> completed : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReady,
				to:          fhir.TaskStatusCompleted,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		{
			name: "ready -> failed : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReady,
				to:          fhir.TaskStatusFailed,
				isOwner:     true,
				isRequester: false,
			},
			want: true,
		},
		// Negative cases -> Invalid requester/owner
		{
			name: "requested -> received : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusReceived,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "requested -> accepted : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusAccepted,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "requested -> rejected : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusRequested,
				to:          fhir.TaskStatusRejected,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "received -> accepted : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusAccepted,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "received -> rejected : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusReceived,
				to:          fhir.TaskStatusRejected,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "accepted -> in-progress : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusAccepted,
				to:          fhir.TaskStatusInProgress,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "in-progress -> completed : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusCompleted,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "in-progress -> failed : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusInProgress,
				to:          fhir.TaskStatusFailed,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "ready -> completed : requester (FAIL)",
			args: args{
				from:        fhir.TaskStatusReady,
				to:          fhir.TaskStatusCompleted,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
		{
			name: "ready -> failed : owner (OK)",
			args: args{
				from:        fhir.TaskStatusReady,
				to:          fhir.TaskStatusFailed,
				isOwner:     false,
				isRequester: true,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidTransition(tt.args.from, tt.args.to, tt.args.isOwner, tt.args.isRequester)
			require.Equal(t, tt.want, got)
			// Validate that the opposite direction fails, besides in-progress <-> on-hold
			if (tt.args.from == fhir.TaskStatusOnHold && tt.args.to == fhir.TaskStatusInProgress) || (tt.args.from == fhir.TaskStatusInProgress && tt.args.to == fhir.TaskStatusOnHold) {
				return
			}
			got = isValidTransition(tt.args.to, tt.args.from, tt.args.isOwner, tt.args.isRequester)
			require.Equal(t, false, got)
		})
	}
}
