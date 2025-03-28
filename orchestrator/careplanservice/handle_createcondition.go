package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating Condition")
	var condition fhir.Condition
	if err := json.Unmarshal(request.ResourceData, &condition); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", condition, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &condition); err != nil {
		return nil, err
	}

	// Verify the requester is the same as the local identity
	if !isRequesterLocalCareOrganization([]fhir.Organization{{Identifier: []fhir.Identifier{*request.LocalIdentity}}}, *request.Principal) {
		return nil, coolfhir.NewErrorWithCode("Only the local care organization can create a Condition", http.StatusForbidden)
	}

	// TODO: Field validation

	conditionBundleEntry := request.bundleEntryWithResource(condition)
	if conditionBundleEntry.FullUrl == nil {
		conditionBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	idx := len(tx.Entry)

	// If condition has an ID, treat as PUT operation
	if condition.Id != nil && request.Upsert {
		tx.Append(condition, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "Condition/" + *condition.Id,
		}, nil, coolfhir.WithFullUrl(*conditionBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	} else {
		tx.Create(condition, coolfhir.WithFullUrl(*conditionBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: request.LocalIdentity,
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdCondition fhir.Condition
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdCondition)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process Condition creation result: %w", err)
		}

		return result, []any{&createdCondition}, nil
	}, nil
}
