package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
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

var _ FHIROperation = &FHIRSearchOperationHandler[any]{}

type FHIRSearchOperationHandler[T any] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy
}

func (h FHIRSearchOperationHandler[T]) Type() FHIROperationType {
	return FHIROperationSearch
}

func (h FHIRSearchOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := request.ResourcePath
	log.Ctx(ctx).Info().Msgf("Searching for %s", resourceType)
	resources, bundle, err := searchResources[T](ctx, h.fhirClient, resourceType, request.QueryParams, request.FhirHeaders)
	if err != nil {
		return nil, err
	}

	// Filter authorized resources
	j := 0
	for i, resource := range resources {
		resourceID := *coolfhir.ResourceID(resource)
		hasAccess, err := h.authzPolicy.HasAccess()
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking authz policy for %s/%s", resourceType, resourceID)
			continue
		}
		if hasAccess {
			resources[j] = resource
			bundle.Entry[j] = bundle.Entry[i]
			j++
		}
	}
	resources = resources[:j]
	bundle.Entry = bundle.Entry[:j]

	results := []*fhir.BundleEntry{}
	for i, entry := range bundle.Entry {
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
		resourceID := coolfhir.ResourceID(resources[i])
		auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
			Id:        resourceID,
			Type:      to.Ptr(resourceType),
			Reference: to.Ptr(resourceType + "/" + *resourceID),
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

func searchResources[T any](ctx context.Context, fhirClient fhirclient.Client, resourceType string, queryParams url.Values, headers *fhirclient.Headers) ([]T, *fhir.Bundle, error) {
	form := url.Values{}
	for k, v := range queryParams {
		form.Add(k, strings.Join(v, ","))
	}

	var bundle fhir.Bundle
	err := fhirClient.SearchWithContext(ctx, resourceType, form, &bundle, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	var resources []T
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType(resourceType), &resources)
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	return resources, &bundle, nil
}
