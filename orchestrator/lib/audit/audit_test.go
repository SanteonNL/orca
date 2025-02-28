package audit

import (
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestAuditEvent(t *testing.T) {
	// Setup
	fixedTime := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	restore := SetNowFuncForTest(func() time.Time { return fixedTime })
	defer restore()

	tests := []struct {
		name           string
		action         fhir.AuditEventAction
		resourceRef    *fhir.Reference
		actingAgentRef *fhir.Reference
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Event(auth.TestPrincipal2.Organization.Identifier[0], tt.action, tt.resourceRef, tt.actingAgentRef)

			assert.Equal(t, fixedTime.Format(time.RFC3339), got.Recorded)
			assert.Equal(t, tt.action, *got.Action)
			assert.Equal(t, tt.resourceRef, got.Entity[0].What)
			assert.Equal(t, tt.actingAgentRef, got.Agent[0].Who)
		})
	}
}
