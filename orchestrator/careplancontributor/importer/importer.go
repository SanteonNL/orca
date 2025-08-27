package _import

import (
	"context"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplanservice"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Import(ctx context.Context, cpsFHIRClient fhirclient.Client,
	taskRequesterOrg fhir.Organization, taskPerformerOrg fhir.Organization, patientIdentifier fhir.Identifier,
	externalIdentifier fhir.Identifier, serviceRequestCode fhir.Coding, conditionCode fhir.Coding, startDate time.Time) (*fhir.Bundle, error) {
	requesterOrgRef := &fhir.Reference{
		Type:       to.Ptr("Organization"),
		Identifier: &taskRequesterOrg.Identifier[0],
		Display:    taskRequesterOrg.Name,
	}
	performerOrgRef := fhir.Reference{
		Type:       to.Ptr("Organization"),
		Identifier: &taskPerformerOrg.Identifier[0],
		Display:    taskPerformerOrg.Name,
	}
	patientRef := fhir.Reference{
		Type:       to.Ptr("Patient"),
		Identifier: &patientIdentifier,
		Reference:  to.Ptr("urn:uuid:patient"),
	}

	tx := coolfhir.Transaction()
	//
	// CarePlan
	//
	carePlan := fhir.CarePlan{
		Id:     to.Ptr("urn:uuid:careplan"),
		Author: requesterOrgRef,
		Category: []fhir.CodeableConcept{
			{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://snomed.info/sct"),
						Code:    to.Ptr("135411000146103"),
						Display: to.Ptr("Multidisciplinary care regime"),
					},
				},
			},
		},
		Intent:  fhir.CarePlanIntentOrder,
		Status:  fhir.RequestStatusActive,
		Subject: patientRef,
		Activity: []fhir.CarePlanActivity{
			{
				Reference: &fhir.Reference{
					Type:      to.Ptr("Task"),
					Reference: to.Ptr("urn:uuid:task"),
				},
			},
		},
		CareTeam: []fhir.Reference{
			{
				Type:      to.Ptr("CareTeam"),
				Reference: to.Ptr("#cps-careteam"),
			},
		},
		Contained: must.MarshalJSON([]any{
			fhir.CareTeam{
				Id: to.Ptr("cps-careteam"),
				Participant: []fhir.CareTeamParticipant{
					{
						Member: requesterOrgRef,
						Period: &fhir.Period{
							Start: to.Ptr(startDate.Format(time.RFC3339)),
						},
					},
					{
						Member: &performerOrgRef,
						Period: &fhir.Period{
							Start: to.Ptr(startDate.Format(time.RFC3339)),
						},
					},
				},
			},
		}),
	}
	careplanservice.SetCreatorExtensionOnResource(&carePlan, requesterOrgRef.Identifier)
	tx = tx.Create(carePlan)
	//
	// ServiceRequest
	//
	serviceRequest := fhir.ServiceRequest{
		Id: to.Ptr("urn:uuid:servicerequest"),
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{serviceRequestCode},
		},
		Identifier: []fhir.Identifier{externalIdentifier},
		Intent:     fhir.RequestIntentProposal,
		Status:     fhir.RequestStatusActive,
		Performer:  []fhir.Reference{performerOrgRef},
		Requester:  requesterOrgRef,
		Subject:    patientRef,
		ReasonCode: []fhir.CodeableConcept{
			{
				Coding: []fhir.Coding{conditionCode},
			},
		},
	}
	careplanservice.SetCreatorExtensionOnResource(&serviceRequest, requesterOrgRef.Identifier)
	tx = tx.Create(serviceRequest)
	//
	// Task
	//
	task := fhir.Task{
		Id: to.Ptr("urn:uuid:task"),
		BasedOn: []fhir.Reference{
			{
				Type:      to.Ptr("CarePlan"),
				Reference: to.Ptr("urn:uuid:careplan"),
			},
		},
		Focus: &fhir.Reference{
			Type:      to.Ptr("ServiceRequest"),
			Display:   serviceRequestCode.Display,
			Reference: to.Ptr("urn:uuid:servicerequest"),
		},
		For:        &patientRef,
		Identifier: []fhir.Identifier{externalIdentifier},
		Intent:     "order",
		Meta: &fhir.Meta{
			Profile: []string{
				"http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask",
			},
		},
		Owner:     &performerOrgRef,
		Requester: requesterOrgRef,
		ReasonCode: &fhir.CodeableConcept{
			Coding: []fhir.Coding{conditionCode},
		},
		Status: fhir.TaskStatusInProgress,
	}
	careplanservice.SetCreatorExtensionOnResource(&task, requesterOrgRef.Identifier)
	tx = tx.Create(task)

	// Perform
	var result fhir.Bundle
	if err := cpsFHIRClient.CreateWithContext(ctx, tx.Bundle(), &result, fhirclient.AtPath("/$import")); err != nil {
		return nil, err
	}
	return &result, nil
}
