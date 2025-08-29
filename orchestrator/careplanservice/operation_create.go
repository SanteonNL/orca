package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/observability"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/SanteonNL/orca/orchestrator/lib/validation"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
)

var _ FHIROperation = &FHIRCreateOperationHandler[fhir.HasExtension]{}

type FHIRCreateOperationHandler[T fhir.HasExtension] struct {
	fhirClientFactory FHIRClientFactory
	authzPolicy       Policy[T]
	profile           profile.Provider
	validator         validation.Validator[T]
}

func (h FHIRCreateOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	resourceType := getResourceType(request.ResourcePath)
	span.SetAttributes(
		attribute.String(observability.FHIRResourceType, resourceType),
		attribute.String(observability.OperationName, "Create"),
	)

	var resource T
	if err := json.Unmarshal(request.ResourceData, &resource); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal resource")
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("invalid %s: %v", resourceType, err),
			StatusCode: http.StatusBadRequest,
		}
	}

	resourceID := coolfhir.ResourceID(resource)
	if resourceID != nil {
		span.SetAttributes(attribute.String(observability.FHIRResourceID, *resourceID))
	}

	// Check we're only allowing secure external literal references
	if err := validateLiteralReferences(ctx, h.profile, &resource); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "literal reference validation failed")
		return nil, err
	}

	authzDecision, err := h.authzPolicy.HasAccess(ctx, resource, *request.Principal)
	if authzDecision == nil || !authzDecision.Allowed {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization check failed")
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if principal is authorized to create %s", resourceType)
		} else {
			err := fmt.Errorf("participant is not authorized to create %s", resourceType)
			span.RecordError(err)
			span.SetStatus(codes.Error, "authorization denied")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("Participant is not authorized to create %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	// Add authorization decision details to span
	span.SetAttributes(
		attribute.Bool(observability.AuthZAllowed, authzDecision.Allowed),
		attribute.StringSlice(observability.AuthZReasons, authzDecision.Reasons),
	)

	log.Ctx(ctx).Info().Msgf("Creating %s (authz=%s)", resourceType, strings.Join(authzDecision.Reasons, ";"))

	if h.validator != nil {
		result, err2, done := h.validate(ctx, resource, resourceType)
		if done {
			return result, err2
		}
		span.SetAttributes(attribute.String(observability.ValidationResult, "passed"))
	} else {
		span.SetAttributes(attribute.Bool("validation.enabled", false))
	}

	resourceBundleEntry := request.bundleEntryWithResource(resource)
	if resourceBundleEntry.FullUrl == nil {
		resourceBundleEntry.FullUrl = to.Ptr("urn:uuid:" + uuid.NewString())
	}
	idx := len(tx.Entry)

	SetCreatorExtensionOnResource(resource, &request.Principal.Organization.Identifier[0])

	// If the resource has an ID and the upsert flag is set, treat as PUT operation
	// As per FHIR spec, this is how we can create a resource with a client supplied ID: https://hl7.org/fhir/http.html#upsert
	if resourceID != nil && request.Upsert {
		span.SetAttributes(attribute.String("fhir.operation.mode", "upsert"))

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
		span.SetAttributes(attribute.String("fhir.operation.mode", "create"))

		tx.Create(resource, coolfhir.WithFullUrl(*resourceBundleEntry.FullUrl), coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
			ActingAgent: &fhir.Reference{
				Identifier: &request.Principal.Organization.Identifier[0],
				Type:       to.Ptr("Organization"),
			},
			Observer: *request.LocalIdentity,
			Action:   fhir.AuditEventActionC,
		}))
	}

	fhirClient, err := h.fhirClientFactory(ctx)
	if err != nil {
		return nil, err
	}
	span.SetStatus(codes.Ok, "")
	span.SetAttributes(
		attribute.String("fhir.resource.creation", "success"),
	)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		var createdResource T
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, fhirClient, request.BaseURL, &tx.Entry[idx], &txResult.Entry[idx], &createdResource)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process %s creation result: %w", resourceType, err)
		}

		return []*fhir.BundleEntry{result}, []any{createdResource}, nil
	}, nil
}

func (h FHIRCreateOperationHandler[T]) validate(ctx context.Context, resource T, resourceType string) (FHIRHandlerResult, error, bool) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(observability.FHIRResourceType, resourceType),
		),
	)
	defer span.End()

	if errs := h.validator.Validate(resource); errs != nil {
		span.SetAttributes(
			attribute.Int("validation.error_count", len(errs)),
			attribute.String(observability.ValidationResult, "failed"),
		)

		var issues []fhir.OperationOutcomeIssue
		var diagnostics = fmt.Sprintf("Validation failed for %s", resourceType)
		var codings []fhir.Coding
		for _, err := range errs {
			var coding = fhir.Coding{
				Code:   to.Ptr(err.Code),
				System: to.Ptr("https://zorgbijjou.github.io/scp-homemonitoring/validation/"),
			}
			codings = append(codings, coding)
		}

		issues = append(issues, fhir.OperationOutcomeIssue{
			Severity:    fhir.IssueSeverityError,
			Code:        fhir.IssueTypeInvariant,
			Diagnostics: &diagnostics,
			Details: &fhir.CodeableConcept{
				Coding: codings,
			},
		})

		var err = &fhirclient.OperationOutcomeError{
			OperationOutcome: fhir.OperationOutcome{
				Issue: issues,
			},
			HttpStatusCode: http.StatusBadRequest,
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "validation failed")
		return nil, err, true
	}

	span.SetAttributes(
		attribute.String(observability.ValidationResult, "passed"),
	)
	span.SetStatus(codes.Ok, "")
	return nil, nil, false
}
