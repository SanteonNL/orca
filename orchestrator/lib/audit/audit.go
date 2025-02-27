package audit

import (
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/to"

	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var nowFunc = time.Now

type Config struct {
	AuditObserverSystem string
	AuditObserverValue  string
}

var config Config

func Configure(cfg Config) {
	config = cfg
}

func Event(action fhir.AuditEventAction, resourceReference *fhir.Reference, actingAgentRef *fhir.Reference) *fhir.AuditEvent {
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
		Recorded: nowFunc().Format(time.RFC3339),
		Action:   to.Ptr(fhir.AuditEventAction(action)),
		Source: fhir.AuditEventSource{
			Observer: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr(config.AuditObserverSystem),
					Value:  to.Ptr(config.AuditObserverValue),
				},
				Type: to.Ptr("Device"),
			},
		},
	}

	return &auditEvent
}
