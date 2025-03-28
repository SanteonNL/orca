package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleUpdateServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Updating ServiceRequest: %s", request.RequestUrl)
	var serviceRequest fhir.ServiceRequest
	if err := json.Unmarshal(request.ResourceData, &serviceRequest); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", serviceRequest, coolfhir.BadRequestError(err))
	}

	// Check we're only allowing secure external literal references
	if err := s.validateLiteralReferences(ctx, &serviceRequest); err != nil {
		return nil, err
	}

	// Search for the existing ServiceRequest
	var searchBundle fhir.Bundle

	serviceRequestId := ""
	if serviceRequest.Id != nil {
		serviceRequestId = *serviceRequest.Id
	}

	if serviceRequestId != "" {
		err := s.fhirClient.SearchWithContext(ctx, "ServiceRequest", url.Values{
			"_id": []string{serviceRequestId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for ServiceRequest: %w", err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		log.Ctx(ctx).Info().Msgf("ServiceRequest not found, handling as create: %s", serviceRequestId)
		request.Upsert = true
		return s.handleCreateServiceRequest(ctx, request, tx)
	}

	// Extract the existing ServiceRequest from the bundle
	var existingServiceRequest fhir.ServiceRequest
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingServiceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing ServiceRequest: %w", err)
	}

	isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "ServiceRequest", serviceRequestId)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of ServiceRequest")
	}
	if !isCreator {
		return nil, coolfhir.NewErrorWithCode("Participant does not have access to ServiceRequest", http.StatusForbidden)
	}

	idx := len(tx.Entry)
	serviceRequestBundleEntry := request.bundleEntryWithResource(serviceRequest)
	tx.AppendEntry(serviceRequestBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: request.LocalIdentity,
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
	}))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedServiceRequest fhir.ServiceRequest
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &serviceRequestBundleEntry, &txResult.Entry[idx], &updatedServiceRequest)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original ServiceRequest that was sent.
			updatedServiceRequest = serviceRequest
		} else if err != nil {
			return nil, nil, err
		}

		return result, []any{&updatedServiceRequest}, nil
	}, nil
}
