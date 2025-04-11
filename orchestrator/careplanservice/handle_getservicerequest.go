package careplanservice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleReadServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting ServiceRequest with ID: %s", request.ResourceId)
	var serviceRequest fhir.ServiceRequest
	err := s.fhirClient.ReadWithContext(ctx, "ServiceRequest/"+request.ResourceId, &serviceRequest, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	// TODO: This query is going to become more expensive as we create more tasks, we should look at setting ServiceRequest.BasedOn or another field to the Task ID
	// If Task validation passes, the user has access to the ServiceRequest
	bundle, err := s.searchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *serviceRequest.Id}}, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}
	// If the user does not have access to the Task, check if they are the creator of the ServiceRequest
	if len(bundle.Entry) == 0 {
		// If the user created the service request, they have access to it
		isCreator, err := s.isCreatorOfResource(ctx, *request.Principal, "ServiceRequest", request.ResourceId)
		if isCreator {
			// User has access, continue with transaction
		} else {
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msg("Error checking if user is creator of ServiceRequest")
			}

			return nil, &coolfhir.ErrorWithCode{
				Message:    "Participant does not have access to ServiceRequest",
				StatusCode: http.StatusForbidden,
			}
		}
	}

	serviceRequestRaw, err := json.Marshal(serviceRequest)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: serviceRequestRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        serviceRequest.Id,
		Type:      to.Ptr("ServiceRequest"),
		Reference: to.Ptr("ServiceRequest/" + *serviceRequest.Id),
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

// handleSearchServiceRequest performs a search for ServiceRequest based on the user request parameters
// and filters the results based on user authorization
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchServiceRequest(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Searching for ServiceRequests")

	bundle, err := s.searchServiceRequest(ctx, request.QueryParams, request.FhirHeaders, *request.Principal)
	if err != nil {
		return nil, err
	}

	results := []*fhir.BundleEntry{}

	for _, entry := range bundle.Entry {
		var currentServiceRequest fhir.ServiceRequest
		if err := json.Unmarshal(entry.Resource, &currentServiceRequest); err != nil {
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
			Id:        currentServiceRequest.Id,
			Type:      to.Ptr("ServiceRequest"),
			Reference: to.Ptr("ServiceRequest/" + *currentServiceRequest.Id),
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

// searchServiceRequest performs the core functionality of searching for service requests and filtering by authorization
// This can be used by other resources to search for service requests and filter by authorization
func (s *Service) searchServiceRequest(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers, principal auth.Principal) (*fhir.Bundle, error) {
	serviceRequests, bundle, err := handleSearchResource[fhir.ServiceRequest](ctx, s, "ServiceRequest", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(serviceRequests) == 0 {
		// If there are no service requests in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	// For each ServiceRequest, check if the user has access via Task or as creator
	var allowedServiceRequestRefs []string
	for _, sr := range serviceRequests {
		// Try to find a Task that references this ServiceRequest
		taskBundle, err := s.searchTask(ctx, map[string][]string{"focus": {"ServiceRequest/" + *sr.Id}}, headers, principal)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking tasks for ServiceRequest/%s", *sr.Id)
			continue
		}

		// If task is found, the user has access
		if len(taskBundle.Entry) > 0 {
			allowedServiceRequestRefs = append(allowedServiceRequestRefs, "ServiceRequest/"+*sr.Id)
			continue
		}

		// If no task is found, check if the user is the creator
		isCreator, err := s.isCreatorOfResource(ctx, principal, "ServiceRequest", *sr.Id)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Error checking if user is creator of ServiceRequest/%s", *sr.Id)
			continue
		}

		if isCreator {
			allowedServiceRequestRefs = append(allowedServiceRequestRefs, "ServiceRequest/"+*sr.Id)
		}
	}

	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"ServiceRequest"}, allowedServiceRequestRefs)

	return &retBundle, nil
}
