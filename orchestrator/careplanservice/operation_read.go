package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

var _ FHIROperation = &FHIRReadOperationHandler[any]{}

type FHIRReadOperationHandler[T any] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
}

func (h FHIRReadOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	log.Ctx(ctx).Info().Msgf("Getting %s with ID: %s", resourceType, request.ResourceId)
	var resource T
	err := h.fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	hasAccess, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if !hasAccess {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal has access to %s", resourceType)
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant does not have access to %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	resourceRaw, err := json.Marshal(resource)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: resourceRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	resourceID := *coolfhir.ResourceID(resource)
	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        &resourceID,
		Type:      to.Ptr(resourceType),
		Reference: to.Ptr(resourceType + "/" + resourceID),
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
