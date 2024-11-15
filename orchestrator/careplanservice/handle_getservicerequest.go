package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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

	err = s.handleTaskBasedResourceAuth(ctx, headers, serviceRequest.BasedOn, "ServiceRequest")
	if err != nil {
		return nil, err
	}

	return &serviceRequest, nil
}
