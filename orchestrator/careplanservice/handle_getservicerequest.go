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
	log.Ctx(ctx).Info().Str("serviceRequestId", request.ResourceId).Msg("Handling get service request")

	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+request.ResourceId, &serviceRequest)
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.handleSearchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, &fhirclient.Headers{})
	if err != nil {
		return nil, err
	}

	// If the user does not have access to the Task, check if they are the creator of the ServiceRequest
	if len(bundle.Entry) == 0 {
		// If the user created the service request, they have access to it
		isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "ServiceRequest", request.ResourceId)
		if !isCreator || err != nil {
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
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionR,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var returnedServiceRequest fhir.ServiceRequest
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[0], &txResult.Entry[0], &returnedServiceRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get ServiceRequest: %w", err)
		}

		return result, []any{&returnedServiceRequest}, nil
	}, nil
}
