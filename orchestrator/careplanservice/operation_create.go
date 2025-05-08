package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

var _ FHIROperation = &FHIRCreateOperationHandler[any]{}

type FHIRCreateOperationHandler[T any] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
	profile     profile.Provider
	fhirURL     *url.URL
}

func (h FHIRCreateOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	var resource T
	if err := json.Unmarshal(request.ResourceData, &resource); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", resourceType, coolfhir.BadRequestError(err))
	}
	resourceID := coolfhir.ResourceID(resource)
	// Check we're only allowing secure external literal references
	if err := validateLiteralReferences(ctx, h.profile, &resource); err != nil {
		return nil, err
	}
	authzDecision, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal is authorized to create %s", resourceType)
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant is not authorized to create %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}
	log.Ctx(ctx).Info().Msgf("Creating %s (authz=%s)", resourceType, strings.Join(authzDecision.Reasons, ";"))
	// TODO: Field validation
	resourceBundleEntry := request.bundleEntryWithResource(resource)
	if resourceBundleEntry.FullUrl == nil {
		resourceBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}
	idx := len(tx.Entry)

	// Set the creator of the resource in the resource.Extension. This is used for creator-based auth
	if err := appendCreatorExtension(&resource, request.Principal.Organization.Identifier[0]); err != nil {
		return nil, fmt.Errorf("failed to set creator extension: %w", err)
	}

	// If the resource has an ID and the upsert flag is set, treat as PUT operation
	// As per FHIR spec, this is how we can create a resource with a client supplied ID: https://hl7.org/fhir/http.html#upsert
	if resourceID != nil && request.Upsert {
		tx.Append(resource, &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPUT,
			Url:    resourceType + "/" + *resourceID,
		}, nil, coolfhir.WithFullUrl(*resourceBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
			Policy:   authzDecision.Reasons,
		}))
	} else {
		tx.Create(resource, coolfhir.WithFullUrl(*resourceBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var createdResource T
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, h.fhirClient, h.fhirURL, &tx.Entry[idx], &txResult.Entry[idx], &createdResource)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process %s creation result: %w", resourceType, err)
		}

		return []*fhir.BundleEntry{result}, []any{&createdResource}, nil
	}, nil
}

// appendCreatorExtension adds the creator organization identifier to the resource's Extension array
func appendCreatorExtension(resource interface{}, identifier fhir.Identifier) error {
	val := reflect.ValueOf(resource)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return fmt.Errorf("resource must be a non-nil pointer")
	}

	elem := val.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("resource must be a pointer to a struct")
	}

	extensionField := elem.FieldByName("Extension")
	if !extensionField.IsValid() {
		return fmt.Errorf("resource has no Extension field")
	}

	creatorExtension := fhir.Extension{
		Url: CreatorExtensionURL,
		ValueReference: &fhir.Reference{
			Identifier: &identifier,
			Type:       to.Ptr("Organization"),
		},
	}

	extensionField.Set(reflect.Append(extensionField, reflect.ValueOf(creatorExtension)))
	return nil
}
