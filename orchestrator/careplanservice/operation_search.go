package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"strconv"
	"strings"
)

var _ FHIROperation = &FHIRSearchOperationHandler[any]{}

type FHIRSearchOperationHandler[T any] struct {
	fhirClient  fhirclient.Client
	authzPolicy Policy[T]
}

func (h FHIRSearchOperationHandler[T]) Handle(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	resourceType := getResourceType(request.ResourcePath)
	log.Ctx(ctx).Info().Msgf("Searching for %s", resourceType)
	resources, bundle, err := h.searchAndFilter(ctx, request.QueryParams, request.Principal, resourceType)
	if err != nil {
		return nil, err
	}

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

func (h FHIRSearchOperationHandler[T]) searchAndFilter(ctx context.Context, queryParams url.Values, principal *auth.Principal, resourceType string) ([]T, *fhir.Bundle, error) {
	resources, bundle, err := searchResources[T](ctx, h.fhirClient, resourceType, queryParams, new(fhirclient.Headers))
	if err != nil {
		return nil, nil, err
	}

	// Filter authorized resources
	j := 0
	for i, resource := range resources {
		resourceID := *coolfhir.ResourceID(resource)
		hasAccess, err := h.authzPolicy.HasAccess(ctx, resource, *principal)
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
	return resources, bundle, nil
}

func searchResources[T any](ctx context.Context, fhirClient fhirclient.Client, resourceType string, queryParams url.Values, headers *fhirclient.Headers) ([]T, *fhir.Bundle, error) {
	form := url.Values{}
	for k, v := range queryParams {
		form.Add(k, strings.Join(v, ","))
	}

	var searchLimit int
	if form.Has("_count") {
		count, err := strconv.Atoi(form.Get("_count"))
		if err != nil {
			return nil, &fhir.Bundle{}, fmt.Errorf("invalid _count value: %w", err)
		}
		searchLimit = count
	} else {
		// If we're missing a resource due to too low page count, we might incorrectly deny access
		searchLimit = 10000
		form.Add("_count", strconv.Itoa(searchLimit))
	}

	var bundle fhir.Bundle
	err := fhirClient.SearchWithContext(ctx, resourceType, form, &bundle, fhirclient.ResponseHeaders(headers))
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	// If there's more search results we didn't use, make sure we log this
	searchHasNext := false
	for _, link := range bundle.Link {
		if link.Relation == "next" {
			searchHasNext = true
			break
		}
	}
	if searchHasNext ||
		len(bundle.Entry) > searchLimit-1 ||
		(bundle.Total != nil && *bundle.Total > searchLimit) {
		log.Ctx(ctx).Warn().Msgf("Too many results found for %s search, only the first %d will taken into account. This could lead to not being granted access, or search results being omitted.", resourceType, searchLimit)
	}

	var resources []T
	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType(resourceType), &resources)
	if err != nil {
		return nil, &fhir.Bundle{}, err
	}

	return resources, &bundle, nil
}
