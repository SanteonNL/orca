package careplanservice

import (
	"context"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleGetServiceRequest(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.ServiceRequest, error) {
	// Verify requester is authenticated
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var serviceRequest fhir.ServiceRequest
	err = s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+id, &serviceRequest, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.handleSearchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, headers)
	if err != nil {
		return nil, err
	}
	// If the user does not have access to the Task, check if they are the creator of the ServiceRequest
	if len(bundle.Entry) == 0 {

		// If the user created the service request, they have access to it
		isCreator, err := s.isCreatorOfResource(ctx, principal, "ServiceRequest", id)
		if isCreator {
			return &serviceRequest, nil
		}
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of ServiceRequest")
		}

		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to ServiceRequest",
			StatusCode: http.StatusForbidden,
		}
	}

	return &serviceRequest, nil
}
