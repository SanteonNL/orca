package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

type Policy[T any] interface {
	HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error)
}

var _ Policy[any] = &AnyMatchPolicy[any]{}

// AnyMatchPolicy is a policy that allows access if any of the policies in the list allow access.
type AnyMatchPolicy[T any] struct {
	Policies []Policy[T]
}

func (e AnyMatchPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	for _, policy := range e.Policies {
		hasAccess, err := policy.HasAccess(ctx, resource, principal)
		if err != nil {
			return false, err
		}
		if hasAccess {
			return true, nil
		}
	}
	return false, nil
}

var _ Policy[any] = &RelatedResourcePolicy[any, any]{}

// RelatedResourcePolicy is a policy that allows access if the user has access to the related resource(s).
// For instance, if the user has access to a ServiceRequest, if the user has access to the related Task.
type RelatedResourcePolicy[T any, R any] struct {
	fhirClient          fhirclient.Client
	searchHandlerPolicy Policy[R]
	searchHandlerParams func(ctx context.Context, resource T) (resourceType string, searchParams url.Values)
}

func (r RelatedResourcePolicy[T, R]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	resourceType, searchParams := r.searchHandlerParams(ctx, resource)
	searchHandler := FHIRSearchOperationHandler[R]{
		fhirClient:  r.fhirClient,
		authzPolicy: r.searchHandlerPolicy,
	}
	results, _, err := searchHandler.searchAndFilter(ctx, searchParams, &principal, resourceType)
	if err != nil {
		return false, fmt.Errorf("related resource search (related resource type=%s): %w", resourceType, err)
	}
	return len(results) > 0, nil
}

var _ Policy[fhir.Task] = &TaskOwnerOrRequesterPolicy[fhir.Task]{}

// TaskOwnerOrRequesterPolicy is a policy that allows access if the user is the owner of the task or the requester of the task.
type TaskOwnerOrRequesterPolicy[T fhir.Task] struct {
}

func (t TaskOwnerOrRequesterPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	resourceAsTask, ok := any(resource).(fhir.Task)
	if !ok {
		return false, fmt.Errorf("resource is not a Task")
	}
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&resourceAsTask, principal.Organization.Identifier)
	return isOwner || isRequester, nil
}

var _ Policy[any] = &CareTeamMemberPolicy[any]{}

// CareTeamMemberPolicy is a policy that allows access if the user is a member of the care team.
type CareTeamMemberPolicy[T any] struct {
	fhirClient fhirclient.Client
	// carePlanRefFunc is a function that returns the CarePlan reference for the resource.
	carePlanRefFunc func(resource T) ([]string, error)
}

func (c CareTeamMemberPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	carePlanRefs, err := c.carePlanRefFunc(resource)
	if err != nil {
		return false, fmt.Errorf("CarePlan ref: %w", err)
	}
	for _, carePlanRef := range carePlanRefs {
		var carePlan fhir.CarePlan
		if err := c.fhirClient.ReadWithContext(ctx, carePlanRef, &carePlan); err != nil {
			log.Ctx(ctx).Warn().Msgf("CareTeamMemberPolicy: unable to read CarePlan %s: %v", carePlanRef, err)
			continue
		}

		careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("CareTeamMemberPolicy: unable to derive CareTeam from CarePlan %s: %v", carePlanRef, err)
			continue
		}

		err = validatePrincipalInCareTeam(principal, careTeam)
		if err != nil {
			// only returns error if the principal is not in the care team
			continue
		}
		return true, nil
	}
	return false, nil
}

var _ Policy[any] = &CreatorPolicy[any]{}

type EveryoneHasAccessPolicy[T any] struct {
}

func (e EveryoneHasAccessPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	return true, nil
}

var _ Policy[any] = &EveryoneHasAccessPolicy[any]{}

type CreatorPolicy[T any] struct {
}

func (o CreatorPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	// TODO: Find a more suitable way to handle this auth.
	// The AuditEvent implementation has proven unsuitable and we are using the AuditEvent for unintended purposes.
	// For now, we can return true, as this will follow the same logic as was present before implementing the AuditEvent.

	return true, nil

	//var auditBundle fhir.Bundle
	//err := s.fhirClient.SearchWithContext(ctx, "AuditEvent", url.Values{
	//	"entity": []string{resourceType + "/" + resourceID},
	//	"action": []string{fhir.AuditEventActionC.String()},
	//}, &auditBundle)
	//if err != nil {
	//	return false, fmt.Errorf("failed to find creation AuditEvent: %w", err)
	//}
	//
	//// Check if there's a creation audit event
	//if len(auditBundle.Entry) == 0 {
	//	return false, coolfhir.NewErrorWithCode(fmt.Sprintf("No creation audit event found for %s", resourceType), http.StatusForbidden)
	//}
	//
	//// Get the creator from the audit event
	//var creationAuditEvent fhir.AuditEvent
	//err = json.Unmarshal(auditBundle.Entry[0].Resource, &creationAuditEvent)
	//if err != nil {
	//	return false, fmt.Errorf("failed to unmarshal AuditEvent: %w", err)
	//}
	//
	//// Check if the current user is the creator
	//if !audit.IsCreator(creationAuditEvent, &principal) {
	//	return false, coolfhir.NewErrorWithCode(fmt.Sprintf("Only the creator can update this %s", resourceType), http.StatusForbidden)
	//}
	//
	//return true, nil
}
