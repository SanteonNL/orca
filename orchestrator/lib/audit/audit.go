package audit

import (
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"

	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var nowFunc = time.Now

func Event(localIdentity fhir.Identifier, action fhir.AuditEventAction, resourceReference *fhir.Reference, actingAgentRef *fhir.Reference) *fhir.AuditEvent {
	// Map AuditEventAction to restful-interaction code
	var interactionCode, interactionDisplay string
	switch action {
	case fhir.AuditEventActionC:
		interactionCode = "create"
		interactionDisplay = "Create"
	case fhir.AuditEventActionR:
		interactionCode = "read"
		interactionDisplay = "Read"
	case fhir.AuditEventActionU:
		interactionCode = "update"
		interactionDisplay = "Update"
	default:
		interactionCode = "search"
		interactionDisplay = "Search"
	}

	auditEvent := fhir.AuditEvent{
		Type: fhir.Coding{
			System:  to.Ptr("http://terminology.hl7.org/CodeSystem/audit-event-type"),
			Code:    to.Ptr("rest"),
			Display: to.Ptr("RESTful Operation"),
		},
		Subtype: []fhir.Coding{
			{
				System:  to.Ptr("http://hl7.org/fhir/restful-interaction"),
				Code:    to.Ptr(interactionCode),
				Display: to.Ptr(interactionDisplay),
			},
		},
		Action:   to.Ptr(action),
		Recorded: nowFunc().Format(time.RFC3339),
		// TODO: Allow for failure outcomes when error audits are implemented
		Outcome: to.Ptr(fhir.AuditEventOutcome0),
		Agent: []fhir.AuditEventAgent{
			{
				Who:       actingAgentRef,
				Requestor: true,
			},
		},
		Source: fhir.AuditEventSource{
			Observer: fhir.Reference{
				Identifier: &localIdentity,
				Type:       to.Ptr("Device"),
			},
		},
		Entity: []fhir.AuditEventEntity{
			{
				What: resourceReference,
			},
		},
		Meta: &fhir.Meta{
			VersionId: to.Ptr("1"),
		},
	}

	return &auditEvent
}
