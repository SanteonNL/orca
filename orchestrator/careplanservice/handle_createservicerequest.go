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

func (s *Service) handleCreateServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msg("Creating ServiceRequest")
	var serviceRequest fhir.ServiceRequest
	if err := json.Unmarshal(request.ResourceData, &serviceRequest); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", serviceRequest, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &serviceRequest); err != nil {
		return nil, err
	}

	// Verify the requester is the same as the local identity
	if !isRequesterLocalCareOrganization([]fhir.Organization{{Identifier: []fhir.Identifier{*request.LocalIdentity}}}, *request.Principal) {
		return nil, coolfhir.NewErrorWithCode("Only the local care organization can create a ServiceRequest", http.StatusForbidden)
	}

	// TODO: Field validation

	serviceRequestBundleEntry := request.bundleEntryWithResource(serviceRequest)
	if serviceRequestBundleEntry.FullUrl == nil {
		serviceRequestBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}

	idx := len(tx.Entry)
	// If serviceRequest has an ID, treat as PUT operation
	if serviceRequest.Id != nil && request.HttpMethod == "PUT" {
		tx.Append(serviceRequest, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "ServiceRequest/" + *serviceRequest.Id,
		}, nil, coolfhir.WithFullUrl(*serviceRequestBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	} else {
		tx.Create(serviceRequest, coolfhir.WithFullUrl(*serviceRequestBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var createdServiceRequest fhir.ServiceRequest
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdServiceRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process ServiceRequest creation result: %w", err)
		}

		return []*fhir.BundleEntry{result}, []any{&createdServiceRequest}, nil
	}, nil
}
