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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
	"time"
)

var _ FHIROperation = &FHIRReadOperationHandler[fhir.HasExtension]{}

type FHIRReadOperationHandler[T fhir.HasExtension] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
}

func (h FHIRReadOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	start := time.Now()
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"FHIRReadOperationHandler.Handle",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("operation.name", "ReadResource"),
		),
	)
	defer span.End()

	resourceType := getResourceType(request.ResourcePath)
	span.SetAttributes(
		attribute.String("fhir.resource_type", resourceType),
		attribute.String("fhir.resource_id", request.ResourceId),
	)

	var resource T
	err := h.fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to read resource from FHIR server")
		span.SetAttributes(attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()))
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
		span.SetAttributes(attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()))
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

	resourceRaw, err := json.Marshal(resource)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal resource")
		span.SetAttributes(attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()))
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
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}
