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

func (s *Service) handleGetServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting ServiceRequest with ID: %s", request.ResourceId)
	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+request.ResourceId, &serviceRequest, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.handleSearchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, request.FhirHeaders)
	if err != nil {
		return nil, err
	}
	// If the user does not have access to the Task, check if they are the creator of the ServiceRequest
	if len(bundle.Entry) == 0 {
		// If the user created the service request, they have access to it
		isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "ServiceRequest", request.ResourceId)
		if isCreator {
			// User has access, continue with transaction
		} else {
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of ServiceRequest")
			}

			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to ServiceRequest",
				StatusCode: http.StatusForbidden,
			}
		}
	}

	tx.Get(serviceRequest, "ServiceRequest/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	srEntryIdx := 0

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var retSR fhir.ServiceRequest
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[srEntryIdx], &txResult.Entry[srEntryIdx], &retSR)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process ServiceRequest read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return result, []any{}, nil
	}, nil
}
