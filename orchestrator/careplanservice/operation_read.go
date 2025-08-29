package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/observability"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
)

var _ FHIROperation = &FHIRReadOperationHandler[fhir.HasExtension]{}

type FHIRReadOperationHandler[T fhir.HasExtension] struct {
	fhirClientFactory FHIRClientFactory
	authzPolicy       Policy[T]
}

func (h FHIRReadOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	resourceType := getResourceType(request.ResourcePath)
	span.SetAttributes(
		attribute.String(observability.FHIRResourceType, resourceType),
		attribute.String(observability.FHIRResourceID, request.ResourceId),
		attribute.String(observability.OperationName, "Read"),
	)

	var resource T
	fhirClient, err := h.fhirClientFactory(ctx)
	if err != nil {
		return nil, err
	}
	err = fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read resource from FHIR server")
		return nil, err
	}

	authzDecision, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization check failed")
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal has access to %s", resourceType)
		} else {
			err := fmt.Errorf("participant does not have access to %s", resourceType)
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization denied")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant does not have access to %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	// Add authorization decision details to span
	span.SetAttributes(
		attribute.Bool("fhir.authorization.allowed", authzDecision.Allowed),
		attribute.StringSlice("fhir.authorization.reasons", authzDecision.Reasons),
	)

	log.Ctx(ctx).Info().Msgf("Getting %s/%s (authz=%s)", resourceType, request.ResourceId, strings.Join(authzDecision.Reasons, ";"))

	updateMetaSource(resource, request.BaseURL)

	resourceRaw, err := json.Marshal(resource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal resource")
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
	}, authzDecision.Reasons)
	tx.Create(auditEvent)

	span.SetStatus(codes.Ok, "")
	span.SetAttributes(
		attribute.String("fhir.resource.read", "success"),
	)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}
