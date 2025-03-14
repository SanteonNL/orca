package careplanservice

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleGetCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Str("conditionId", request.ResourceId).Msg("Handling get condition request")

	var condition fhir.Condition
	err := s.fhirClient.ReadWithContext(ctx, "Condition/"+request.ResourceId, &condition)
	if err != nil {
		return nil, err
	}

	// if the condition is for a patient, fetch the patient. If the requester has access to the patient they also have access to the condition
	if condition.Subject.Identifier != nil && condition.Subject.Identifier.System != nil && condition.Subject.Identifier.Value != nil {
		patients, _, err := handleSearchResource[fhir.Patient](ctx, s, "Patient", url.Values{"identifier": {fmt.Sprintf("%s|%s", *condition.Subject.Identifier.System, *condition.Subject.Identifier.Value)}}, request.FHIRHeaders)
		if err != nil {
			return nil, err
		}

		// If we get at least one patient from this, the requester has access to the condition
		patients, err = s.filterAuthorizedPatients(ctx, *request.Principal, patients)
		if err != nil {
			return nil, err
		}

		if len(patients) == 0 {
			isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "Condition", request.ResourceId)
			if !isCreator || err != nil {
				if err != nil {
					log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of Condition")
				}
				return nil, &coolfhir.ErrorWithCode{
					Message:    "Participant does not have access to Condition",
					StatusCode: http.StatusForbidden,
				}
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
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionR,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var returnedCondition fhir.Condition
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[0], &txResult.Entry[0], &returnedCondition)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get Condition: %w", err)
		}

		return result, []any{&returnedCondition}, nil
	}, nil
}
