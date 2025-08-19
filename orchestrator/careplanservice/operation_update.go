package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
	"strings"
)

var _ FHIROperation = &FHIRUpdateOperationHandler[fhir.HasExtension]{}

type FHIRUpdateOperationHandler[T fhir.HasExtension] struct {
	authzPolicy       Policy[T]
	fhirClientFactory FHIRClientFactory
	profile           profile.Provider
	// createHandler is used for upserting
	createHandler *FHIRCreateOperationHandler[T]
}

func (h FHIRUpdateOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	ctx, span := tracer.Start(
		ctx,
		"FHIRUpdateOperationHandler.Handle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("operation.name", "UpdateResource"),
		),
	)
	defer span.End()

	resourceType := getResourceType(request.ResourcePath)
	span.SetAttributes(attribute.String("fhir.resource_type", resourceType))

	var resource T
	if err := json.Unmarshal(request.ResourceData, &resource); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal resource")
		return nil, coolfhir.BadRequest("invalid %s: %s", resourceType, err)
	}

	resourceID := coolfhir.ResourceID(resource)
	if resourceID != nil {
		span.SetAttributes(attribute.String("fhir.resource_id", *resourceID))
	}

	// Check we're only allowing secure external literal references
	if err := validateLiteralReferences(ctx, h.profile, &resource); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "literal reference validation failed")
		return nil, err
	}

	// Search for the existing resource
	var searchBundle fhir.Bundle

	searchParams := make(url.Values)
	if request.ResourceId != "" {
		span.SetAttributes(
			attribute.String("fhir.update.lookup_method", "id"),
			attribute.String("fhir.update.lookup_id", request.ResourceId),
		)

		if coolfhir.ResourceID(resource) != nil && *coolfhir.ResourceID(resource) != request.ResourceId {
			err := coolfhir.BadRequest("resource ID mismatch: %s != %s", *coolfhir.ResourceID(resource), request.ResourceId)
			span.RecordError(err)
			span.SetStatus(codes.Error, "resource ID mismatch")
			return nil, err
		}
		searchParams = url.Values{
			"_id": []string{request.ResourceId},
		}
	} else if len(request.RequestUrl.Query()) > 0 {
		span.SetAttributes(
			attribute.String("fhir.update.lookup_method", "query"),
			attribute.Int("fhir.update.query_param_count", len(request.RequestUrl.Query())),
		)
		searchParams = request.RequestUrl.Query()
	}

	fhirClient, err := h.fhirClientFactory(ctx)
	if err != nil {
		return nil, err
	}
	if len(searchParams) > 0 {
		err := fhirClient.SearchWithContext(ctx, resourceType, searchParams, &searchBundle)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to search for existing resource")
			return nil, fmt.Errorf("failed to search for %s: %w", resourceType, err)
		}
	}

	span.SetAttributes(attribute.Int("fhir.update.search_results", len(searchBundle.Entry)))

	// If no entries found, handle as a create operation
	if len(searchBundle.Entry) == 0 {
		span.SetAttributes(attribute.String("fhir.update.operation_mode", "upsert_create"))
		log.Ctx(ctx).Info().Msgf("%s not found, handling as create: %s", resourceType, searchParams.Encode())
		request.Upsert = true
		return h.createHandler.Handle(ctx, request, tx)
	}

	span.SetAttributes(attribute.String("fhir.update.operation_mode", "update"))

	// Extract the existing resource from the bundle
	var existingResource T
	err = json.Unmarshal(searchBundle.Entry[0].Resource, &existingResource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal existing resource")
		return nil, fmt.Errorf("failed to unmarshal existing %s: %w", resourceType, err)
	}

	authzDecision, err := h.authzPolicy.HasAccess(ctx, existingResource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization check failed")
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal has access to create %s", resourceType)
		} else {
			err := fmt.Errorf("participant is not authorized to update %s", resourceType)
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization denied")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant is not authorized to update %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	// Add authorization decision details to span
	span.SetAttributes(
		attribute.Bool("fhir.authorization.allowed", authzDecision.Allowed),
		attribute.StringSlice("fhir.authorization.reasons", authzDecision.Reasons),
	)

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

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(
		attribute.String("fhir.resource.update", "success"),
	)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var updatedResource T
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, fhirClient, request.BaseURL, &resourceBundleEntry, &txResult.Entry[idx], &updatedResource)
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
