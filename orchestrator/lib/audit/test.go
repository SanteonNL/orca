package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Helper method for testing/validation
func VerifyAuditEventForTest(t *testing.T, entry *fhir.BundleEntry, resourceReference string, action fhir.AuditEventAction, expectedAgent *fhir.Reference) {
	var auditEvent fhir.AuditEvent
	err := json.Unmarshal(entry.Resource, &auditEvent)
	if err != nil {
		t.Fatalf("Failed to unmarshal audit event: %v", err)
		return
	}

	if auditEvent.Action == nil || *auditEvent.Action != action {
		t.Fatalf("Audit event action mismatch: %v", *auditEvent.Action)
		return
	}

	foundAuditEvent := false
	for _, event := range auditEvent.Entity {
		if *event.What.Reference == resourceReference {
			foundAuditEvent = true
			break
		}
	}
	require.True(t, foundAuditEvent, "Audit event not found")

	// Validate fields
	require.Equal(t, expectedAgent.Identifier.System, auditEvent.Agent[0].Who.Identifier.System)
	require.Equal(t, expectedAgent.Identifier.Value, auditEvent.Agent[0].Who.Identifier.Value)
	require.Equal(t, resourceReference, *auditEvent.Entity[0].What.Reference)
}

// SetNowFuncForTest allows tests to override the time function
// It returns a function to restore the original behavior
// This should only be used in tests
func SetNowFuncForTest(f func() time.Time) func() {
	original := nowFunc
	nowFunc = f
	return func() {
		nowFunc = original
	}
}
