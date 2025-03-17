package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/audit"
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

	// Create audit event for the creation
	createAuditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionC,
		&fhir.Reference{
			Reference: serviceRequestBundleEntry.FullUrl,
			Type:      to.Ptr("ServiceRequest"),
		},
		&fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
	)

	// If serviceRequest has an ID, treat as PUT operation
	if serviceRequest.Id != nil {
		tx.Append(serviceRequest, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    "ServiceRequest/" + *serviceRequest.Id,
		}, nil, coolfhir.WithFullUrl("ServiceRequest/"+*serviceRequest.Id))
		createAuditEvent.Entity[0].What.Reference = to.Ptr("ServiceRequest/" + *serviceRequest.Id)
	} else {
		tx.Create(serviceRequest, coolfhir.WithFullUrl(*serviceRequestBundleEntry.FullUrl))
	}

	serviceRequestEntryIdx := len(tx.Entry) - 1
	tx.Create(createAuditEvent)

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var createdServiceRequest fhir.ServiceRequest
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[serviceRequestEntryIdx], &txResult.Entry[serviceRequestEntryIdx], &createdServiceRequest)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process ServiceRequest creation result: %w", err)
		}

		return result, []any{&createdServiceRequest}, nil
	}, nil
}
