package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var _ FHIROperation = &FHIRReadOperationHandler[fhir.HasExtension]{}

type FHIRReadOperationHandler[T fhir.HasExtension] struct {
	fhirClientFactory FHIRClientFactory
	authzPolicy       Policy[T]
}

func (h FHIRReadOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	resourceType := getResourceType(request.ResourcePath)
	span.SetAttributes(
		attribute.String(otel.FHIRResourceType, resourceType),
		attribute.String(otel.FHIRResourceID, request.ResourceId),
		attribute.String(otel.OperationName, "Read"),
	)

	var resource T
	fhirClient, err := h.fhirClientFactory(ctx)
	if err != nil {
		return nil, err
	}
	err = fhirClient.ReadWithContext(ctx, resourceType+"/"+request.ResourceId, &resource, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, otel.Error(span, err, "failed to read resource from FHIR server")
	}

	authzDecision, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			otel.Error(span, err, "authorization check failed")
			slog.ErrorContext(ctx, "Error checking if principal has access to resource",
				slog.String("error", err.Error()),
				slog.String("resource_type", resourceType))
		}
		return nil, otel.Error(span, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant does not have access to %s", resourceType),
			StatusCode: http.StatusForbidden,
		})
	}

	// Add authorization decision details to span
	span.SetAttributes(
		attribute.Bool("fhir.authorization.allowed", authzDecision.Allowed),
		attribute.StringSlice("fhir.authorization.reasons", authzDecision.Reasons),
	)

	slog.InfoContext(ctx, "Getting resource",
		slog.String("resource_type", resourceType),
		slog.String("resource_id", request.ResourceId),
		slog.String("authz", strings.Join(authzDecision.Reasons, ";")))
	updateMetaSource(resource, request.BaseURL)

	resourceRaw, err := json.Marshal(resource)
	if err != nil {
		return nil, otel.Error(span, err, "failed to marshal resource")
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
