package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
)

var _ FHIROperation = &FHIRUpdateOperationHandler[fhir.HasExtension]{}

type FHIRUpdateOperationHandler[T fhir.HasExtension] struct {
	authzPolicy Policy[T]
	fhirClient  fhirclient.Client
	profile     profile.Provider
	// createHandler is used for upserting
	createHandler *FHIRCreateOperationHandler[T]
	fhirURL       *url.URL
}

func (h FHIRUpdateOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	var resource T
	if err := json.Unmarshal(request.ResourceData, &resource); err != nil {
		return nil, coolfhir.BadRequest("invalid %s: %s", resourceType, err)
	}
	resourceID := coolfhir.ResourceID(resource)

	// Check we're only allowing secure external literal references
	if err := validateLiteralReferences(ctx, h.profile, &resource); err != nil {
		return nil, err
	}

	// Search for the existing resource
	var searchBundle fhir.Bundle

	resourceId := ""
	if resourceID != nil {
		resourceId = *resourceID
	}

	if resourceId != "" {
		err := h.fhirClient.SearchWithContext(ctx, resourceType, url.Values{
			"_id": []string{resourceId},
		}, &searchBundle)
		if err != nil {
			return nil, fmt.Errorf("failed to search for %s: %w", resourceType, err)
		}
	}

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		log.Ctx(ctx).Info().Msgf("%s not found, handling as create: %s", resourceType, resourceId)
		request.Upsert = true
		return h.createHandler.Handle(ctx, request, tx)
	}

	// Extract the existing resource from the bundle
	var existingResource T
	err := json.Unmarshal(searchBundle.Entry[0].Resource, &existingResource)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing %s: %w", resourceType, err)
	}

	authzDecision, err := h.authzPolicy.HasAccess(ctx, existingResource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal has access to create %s", resourceType)
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant is not authorized to update %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	SetCreatorExtensionOnResource(resource, &request.Principal.Organization.Identifier[0])

	log.Ctx(ctx).Info().Msgf("Updating %s (authz=%s)", request.RequestUrl, strings.Join(authzDecision.Reasons, ";"))

	idx := len(tx.Entry)
	resourceBundleEntry := request.bundleEntryWithResource(resource)
	tx.AppendEntry(resourceBundleEntry, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
		Action:   fhir.AuditEventActionU,
		Policy:   authzDecision.Reasons,
	}))

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var updatedResource T
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, h.fhirClient, h.fhirURL, &resourceBundleEntry, &txResult.Entry[idx], &updatedResource)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			// Just respond with the original resource that was sent.
			updatedResource = resource
		} else if err != nil {
			return nil, nil, err
		}

		return []*fhir.BundleEntry{result}, []any{updatedResource}, nil
	}, nil
}
