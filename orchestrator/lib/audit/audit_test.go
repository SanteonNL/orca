package audit

import (
	"context"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestAuditEvent(t *testing.T) {
	// Setup
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	TimeNow = func() time.Time { return fixedTime }
	defer func() { TimeNow = time.Now }()

	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []struct {
		name           string
		action         fhir.AuditEventAction
		resourceRef    *fhir.Reference
		actingAgentRef *fhir.Reference
		wantErr        bool
	}{
		{
			name:   "valid audit event creation",
			action: fhir.AuditEventActionC,
			resourceRef: &fhir.Reference{
				Reference: to.Ptr("Task/123"),
				Type:      to.Ptr("Task"),
			},
			actingAgentRef: &fhir.Reference{
				Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			wantErr: false,
		},
		{
			name:   "missing principal in context",
			action: fhir.AuditEventActionC,
			resourceRef: &fhir.Reference{
				Reference: to.Ptr("Task/123"),
			},
			actingAgentRef: &fhir.Reference{},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var testCtx context.Context
			if tt.name == "missing principal in context" {
				testCtx = context.Background()
			} else {
				testCtx = ctx
			}

			got, err := AuditEvent(testCtx, tt.action, tt.resourceRef, tt.actingAgentRef)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, fixedTime.Format(time.RFC3339), got.Recorded)
			assert.Equal(t, tt.action, *got.Action)
			assert.Equal(t, tt.resourceRef, got.Entity[0].What)
			assert.Equal(t, tt.actingAgentRef, got.Agent[0].Who)
		})
	}
}

func TestVerifyAuditEvent(t *testing.T) {
	tests := []struct {
		name           string
		entry          *fhir.BundleEntry
		resourceRef    string
		action         fhir.AuditEventAction
		expectedResult bool
	}{
		{
			name: "matching audit event",
			entry: &fhir.BundleEntry{
				Resource: []byte(`{
					"resourceType": "AuditEvent",
					"action": "C",
					"entity": [{"what": {"reference": "Task/123"}}]
				}`),
			},
			resourceRef:    "Task/123",
			action:         fhir.AuditEventActionC,
			expectedResult: true,
		},
		{
			name: "non-matching action",
			entry: &fhir.BundleEntry{
				Resource: []byte(`{
					"resourceType": "AuditEvent",
					"action": "R",
					"entity": [{"what": {"reference": "Task/123"}}]
				}`),
			},
			resourceRef:    "Task/123",
			action:         fhir.AuditEventActionC,
			expectedResult: false,
		},
		{
			name: "non-matching reference",
			entry: &fhir.BundleEntry{
				Resource: []byte(`{
					"resourceType": "AuditEvent",
					"action": "C",
					"entity": [{"what": {"reference": "Task/456"}}]
				}`),
			},
			resourceRef:    "Task/123",
			action:         fhir.AuditEventActionC,
			expectedResult: false,
		},
		{
			name: "invalid json",
			entry: &fhir.BundleEntry{
				Resource: []byte(`invalid json`),
			},
			resourceRef:    "Task/123",
			action:         fhir.AuditEventActionC,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifyAuditEvent(t, tt.entry, tt.resourceRef, tt.action)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
