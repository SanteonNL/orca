package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func (s *Service) handleGetServiceRequest(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.ServiceRequest, error) {
	// Verify requester is authenticated
	_, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	var serviceRequest fhir.ServiceRequest
	err = s.fhirClient.Read("ServiceRequest/"+id, &serviceRequest, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.handleSearchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, headers)
	if err != nil {
		return nil, err
	}
	// If bundle is empty, the user does not have access to the ServiceRequest
	if len(bundle.Entry) == 0 {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "User does not have access to ServiceRequest",
			StatusCode: http.StatusUnauthorized,
		}
	}

	return &serviceRequest, nil
}
