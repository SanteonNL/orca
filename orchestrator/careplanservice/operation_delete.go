package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"strings"
)

var _ FHIROperation = &FHIRDeleteOperationHandler[fhir.HasExtension]{}

type FHIRDeleteOperationHandler[T fhir.HasExtension] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
}

func (h FHIRDeleteOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	var resource T
	err := h.fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}
	resourceID := request.ResourceId

	authzDecision, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal has access to delete %s", resourceType)
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant is not authorized to delete %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}
	log.Ctx(ctx).Info().Msgf("Deleting %s/%s (authz=%s)", resourceType, resourceID, strings.Join(authzDecision.Reasons, ";"))

	// Delete AuditEvents first
	var auditBundle fhir.Bundle
	err = h.fhirClient.SearchWithContext(ctx, "AuditEvent", url.Values{
		"entity": []string{resourceType + "/" + resourceID},
	}, &auditBundle)
	if err != nil {
		// Log the error but don't return, if it fails we can still delete the resource
		log.Ctx(ctx).Error().Err(err).Msgf("Error searching for AuditEvents for %s/%s", resourceType, resourceID)
	}

	// Delete AuditEvents individually first
	for _, entry := range auditBundle.Entry {
		var auditEvent fhir.AuditEvent
		err := json.Unmarshal(entry.Resource, &auditEvent)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error unmarshaling AuditEvent for %s/%s", resourceType, resourceID)
			continue
		}

		if auditEvent.Id != nil {
			// Use the special $delete operation for AuditEvents
			deleteParams := fhir.Parameters{
				Parameter: []fhir.ParametersParameter{
					{
						Name:        "id",
						ValueString: auditEvent.Id,
					},
				},
			}

			// Add to transaction
			tx.Append(deleteParams, &fhir.BundleEntryRequest{
				Method: fhir.HTTPVerbPOST,
				Url:    "AuditEvent/$delete",
			}, nil)
		}
	}

	// Use the special $delete operation for the main resource
	idx := len(tx.Entry)
	deleteParams := fhir.Parameters{
		Parameter: []fhir.ParametersParameter{
			{
				Name:        "id",
				ValueString: &resourceID,
			},
		},
	}

	// Add to transaction
	tx.Append(deleteParams, &fhir.BundleEntryRequest{
		Method: fhir.HTTPVerbPOST,
		Url:    resourceType + "/$delete",
	}, nil)

	// Return an empty bundle entry to indicate success
	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		bundleEntry := &fhir.BundleEntry{
			Response: &fhir.BundleEntryResponse{
				Status: "204 No Content",
			},
		}

		// Check if we have a response in the transaction result for the main resource
		if idx < len(txResult.Entry) {
			bundleEntry.Response = txResult.Entry[idx].Response
		}

		return []*fhir.BundleEntry{bundleEntry}, []any{}, nil
	}, nil
}
