package audit

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"

	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var TimeNow = time.Now

func AuditEvent(ctx context.Context, action fhir.AuditEventAction, resourceReference *fhir.Reference, actingAgentRef *fhir.Reference) (*fhir.AuditEvent, error) {
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	auditEvent := fhir.AuditEvent{
		Agent: []fhir.AuditEventAgent{
			{
				Who: actingAgentRef,
			},
		},
		Entity: []fhir.AuditEventEntity{
			{
				What: resourceReference,
			},
		},
		Recorded: TimeNow().Format(time.RFC3339),
		Action:   to.Ptr(fhir.AuditEventAction(action)),
		Source: fhir.AuditEventSource{
			// Local organisation i.e. orchestrator
			Observer: fhir.Reference{
				Identifier: &principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
		},
	}

	return &auditEvent, nil
}

// Helper method for testing/validation
func VerifyAuditEvent(t *testing.T, entry *fhir.BundleEntry, resourceReference string, action fhir.AuditEventAction) bool {
	var auditEvent fhir.AuditEvent
	err := json.Unmarshal(entry.Resource, &auditEvent)
	if err != nil {
		return false
	}

	if auditEvent.Action == nil || *auditEvent.Action != action {
		return false
	}

	foundAuditEvent := false
	for _, event := range auditEvent.Entity {
		if *event.What.Reference == resourceReference {
			foundAuditEvent = true
			break
		}
	}
	return foundAuditEvent
}
