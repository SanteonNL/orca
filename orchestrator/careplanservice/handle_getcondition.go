package careplanservice

import (
	"context"
	"fmt"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetCondition fetches the requested Condition and validates if the requester has access to the resource
// by checking if they have access to the Patient referenced in the Condition's subject
// if the requester is valid, return the Condition, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleGetCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
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

	tx.Get(condition, "Condition/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	conditionEntryIdx := 0

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var retCondition fhir.Condition
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[conditionEntryIdx], &txResult.Entry[conditionEntryIdx], &retCondition)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Condition read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{result}, []any{}, nil
	}, nil
}
