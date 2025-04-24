package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

func ReadServiceRequestAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.ServiceRequest] {
	return AnyMatchPolicy[fhir.ServiceRequest]{
		Policies: []Policy[fhir.ServiceRequest]{
			RelatedResourceSearchPolicy[fhir.ServiceRequest, fhir.Task]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadTaskAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.ServiceRequest) (string, *url.Values) {
					return "Task", &url.Values{"focus": []string{"ServiceRequest/" + *resource.Id}}
				},
			},
			CreatorPolicy[fhir.ServiceRequest]{},
		},
	}
}

func (s *Service) handleReadServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting ServiceRequest with ID: %s", request.ResourceId)
	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+request.ResourceId, &serviceRequest, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	canAccess, err := ReadServiceRequestAuthzPolicy(s.fhirClient).HasAccess(ctx, serviceRequest, *request.Principal)
	if err != nil {
		return nil, err
	}
	// If the user does not have access to the Task, check if they are the creator of the ServiceRequest
	if !canAccess {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to ServiceRequest",
			StatusCode: http.StatusForbidden,
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
