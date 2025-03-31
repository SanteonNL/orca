package careplanservice

import (
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var _ events.Type = CarePlanCreatedEvent{}

type CarePlanCreatedEvent struct {
	fhir.CarePlan
}

func (c CarePlanCreatedEvent) Instance() events.Type {
	return &CarePlanCreatedEvent{}
}

func (c CarePlanCreatedEvent) Entity() messaging.Entity {
	return messaging.Entity{
		Name:   "orca.hl7.fhir.careplan-created",
		Prefix: true,
	}
}
