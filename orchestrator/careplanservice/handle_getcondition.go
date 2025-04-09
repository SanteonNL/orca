package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleReadCondition fetches the requested Condition and validates if the requester has access to the resource
// by checking if they have access to the Patient referenced in the Condition's subject
// if the requester is valid, return the Condition, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleReadCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Condition with ID: %s", request.ResourceId)
	var condition fhir.Condition
	err := s.fhirClient.ReadWithContext(ctx, "Condition/"+request.ResourceId, &condition, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: Find out new auth requirements for condition
	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier != nil && condition.Subject.Identifier.System != nil && condition.Subject.Identifier.Value != nil {
		bundle, err := s.searchPatient(ctx, map[string][]string{"identifier": {fmt.Sprintf("%s|%s", *condition.Subject.Identifier.System, *condition.Subject.Identifier.Value)}}, request.FhirHeaders, *request.Principal)
		if err != nil {
			return nil, err
		}
		if len(bundle.Entry) == 0 {
			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to Condition",
				StatusCode: http.StatusForbidden,
			}
		}
	} else {
		log.Ctx(ctx).Warn().Msg("Condition does not have Patient as subject, can't verify access")
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Condition",
			StatusCode: http.StatusForbidden,
		}
	}

	conditionRaw, err := json.Marshal(condition)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: conditionRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        condition.Id,
		Type:      to.Ptr("Condition"),
		Reference: to.Ptr("Condition/" + *condition.Id),
	}, &fhir.Reference{
		Identifier: &request.Principal.Organization.Identifier[0],
		Type:       to.Ptr("Organization"),
	})
	tx.Create(auditEvent)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}
