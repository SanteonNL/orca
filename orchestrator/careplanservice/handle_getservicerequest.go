package careplanservice

import (
	"context"
	"encoding/json"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleReadServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting ServiceRequest with ID: %s", request.ResourceId)
	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+request.ResourceId, &serviceRequest, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.searchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, request.FhirHeaders, *request.Principal)
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

	serviceRequestRaw, err := json.Marshal(serviceRequest)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: serviceRequestRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        serviceRequest.Id,
		Type:      to.Ptr("ServiceRequest"),
		Reference: to.Ptr("ServiceRequest/" + *serviceRequest.Id),
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
