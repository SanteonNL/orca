package careplanservice

import (
	"context"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleGetServiceRequest(ctx context.Context, id string, headers *fhirclient.Headers) (*fhir.ServiceRequest, error) {
	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+id, &serviceRequest, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, err
	}

	return &serviceRequest, nil
}
