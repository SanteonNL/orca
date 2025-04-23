package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"strings"
)

type FHIROperationType string

const (
	FHIROperationCreate FHIROperationType = "create"
	FHIROperationUpdate FHIROperationType = "update"
	FHIROperationRead   FHIROperationType = "read"
	FHIROperationSearch FHIROperationType = "search"
)

type FHIROperation interface {
	Type() FHIROperationType
	Handle(context.Context, FHIRHandlerRequest, *coolfhir.BundleBuilder) (FHIRHandlerResult, error)
}

type FHIRCreateOperation struct {
}

func (o *FHIRCreateOperation) Type() FHIROperationType {
	return FHIROperationCreate
}

var _ FHIROperation = &FHIRSearchOperationHandler{}

type FHIRSearchOperationHandler struct {
	fhirClient fhirclient.Client
}

func (h FHIRSearchOperationHandler) Type() FHIROperationType {
	return FHIROperationSearch
}

func (h FHIRSearchOperationHandler) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := request.ResourcePath
	log.Ctx(ctx).Info().Msgf("Searching for %s", resourceType)
	var bundle fhir.Bundle
	err := h.fhirClient.SearchWithContext(ctx, resourceType, request.QueryParams, &bundle, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}
	for _, entry := range bundle.Entry {
		var currentResource fhir.Resource
		if err := json.Unmarshal(entry.Resource, &currentResource); err != nil {
			log.Ctx(ctx).Error().
				Err(err).
				Msg("Failed to unmarshal resource for audit")
			continue
		}

		// Create the query detail entity
		queryEntity := fhir.AuditEventEntity{
			Type: &fhir.Coding{
				System:  to.Ptr("http://terminology.hl7.org/CodeSystem/audit-entity-type"),
				Code:    to.Ptr("2"), // query parameters
				Display: to.Ptr("Query Parameters"),
			},
			Detail: []fhir.AuditEventEntityDetail{},
		}

		// Add each query parameter as a detail
		for param, values := range request.QueryParams {
			queryEntity.Detail = append(queryEntity.Detail, fhir.AuditEventEntityDetail{
				Type:        param, // parameter name as string
				ValueString: to.Ptr(strings.Join(values, ",")),
			})
		}

		bundleEntry := fhir.BundleEntry{
			Resource: entry.Resource,
			Response: &fhir.BundleEntryResponse{
				Status: "200 OK",
			},
		}
		results = append(results, &bundleEntry)

		// Add audit event to the transaction
		auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
			Id:        currentResource.Id,
			Type:      to.Ptr(resourceType),
			Reference: to.Ptr(resourceType + "/" + *currentResource.Id),
		}, &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		})
		tx.Create(auditEvent)
	}

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// Simply return the already prepared results
		return results, []any{}, nil
	}, nil
}
