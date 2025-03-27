package careplanservice

import (
	"context"
	"fmt"
	"net/url"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// handleGetCarePlan fetches the requested CarePlan and validates if the requester has access to the resource (is a participant of one of the CareTeams of the care plan)
// if the requester is valid, return the CarePlan, else return an error
func (s *Service) handleGetCarePlan(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting CarePlan with ID: %s", request.ResourceId)
	var carePlan fhir.CarePlan

	// fetch CarePlan, validate requester is participant of CareTeam
	err := s.fhirClient.ReadWithContext(ctx, "CarePlan/"+request.ResourceId, &carePlan, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	if err != nil {
		return nil, err
	}

	err = validatePrincipalInCareTeam(*request.Principal, careTeam)
	if err != nil {
		return nil, err
	}

	tx.Get(carePlan, "CarePlan/"+request.ResourceId, coolfhir.WithAuditEvent(ctx, tx, coolfhir.AuditEventInfo{
		Action: fhir.AuditEventActionR,
		ActingAgent: &fhir.Reference{
			Identifier: &request.Principal.Organization.Identifier[0],
			Type:       to.Ptr("Organization"),
		},
		Observer: *request.LocalIdentity,
	}))

	carePlanEntryIdx := 0

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var retCarePlan fhir.CarePlan
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(ctx, s.fhirClient, s.fhirURL, &tx.Entry[carePlanEntryIdx], &txResult.Entry[carePlanEntryIdx], &retCarePlan)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to process CarePlan read result: %w", err)
		}
		// We do not want to notify subscribers for a get
		return result, []any{}, nil
	}, nil
}

// handleSearchCarePlan does a search for CarePlan based on the user requester parameters. If CareTeam is not requested, add this to the fetch to be used for validation
// if the requester is a participant of one of the returned CareTeams, return the whole bundle, else error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleSearchCarePlan(ctx context.Context, queryParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error) {
	// Verify requester is authenticated
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, err
	}

	carePlans, bundle, err := handleSearchResource[fhir.CarePlan](ctx, s, "CarePlan", queryParams, headers)
	if err != nil {
		return nil, err
	}
	if len(carePlans) == 0 {
		// If there are no carePlans in the bundle there is no point in doing validation, return empty bundle to user
		return &fhir.Bundle{Entry: []fhir.BundleEntry{}}, nil
	}

	// For each CareTeam in bundle, validate the requester is a participant, and if not remove it from the bundle
	// This will be done by adding the IDs we do want to keep to a list, and then filtering the bundle based on this list
	filterRefs := make([]string, 0)
	for _, cp := range carePlans {
		ct, err := coolfhir.CareTeamFromCarePlan(&cp)
		if err != nil {
			continue
		}

		err = validatePrincipalInCareTeam(principal, ct)
		if err != nil {
			continue
		}

		filterRefs = append(filterRefs, "CarePlan/"+*cp.Id)
	}

	retBundle := filterMatchingResourcesInBundle(ctx, bundle, []string{"CarePlan"}, filterRefs)

	return &retBundle, nil
}
