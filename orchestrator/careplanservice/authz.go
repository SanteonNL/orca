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
	fhirClient            fhirclient.Client
	relatedResourcePolicy Policy[R]
	relatedResourceRefs   func(ctx context.Context, resource T) ([]string, error)
}

func (r RelatedResourcePolicy[T, R]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	refs, err := r.relatedResourceRefs(ctx, resource)
	if err != nil {
		return false, fmt.Errorf("related resource ref: %w", err)
	}
	for _, ref := range refs {
		var relatedResource R
		if err := r.fhirClient.ReadWithContext(ctx, ref, &relatedResource); err != nil {
			log.Ctx(ctx).Warn().Msgf("RelatedResourcePolicy: unable to read related resource %s: %v", ref, err)
			continue
		}
		hasAccess, err := r.relatedResourcePolicy.HasAccess(ctx, relatedResource, principal)
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("RelatedResourcePolicy: unable to check access to related resource %s: %v", ref, err)
			continue
		}
		if hasAccess {
			return true, nil
		}
	}
	return false, nil
}

var _ Policy[any] = &RelatedResourceSearchPolicy[any, any]{}

// RelatedResourceSearchPolicy is a policy that allows access if the user has access to the related resource(s).
// For instance, if the user has access to a ServiceRequest, if the user has access to the related Task.
// It differs from RelatedResourcePolicy in that it uses a search operation to find the related resources,
// instead of using a reference to the related resource.
type RelatedResourceSearchPolicy[T any, R any] struct {
	fhirClient            fhirclient.Client
	relatedResourcePolicy Policy[R]
	// relatedResourceSearchParams is a function that returns the search parameters for the related resource.
	// If the resource lacks a reference to the related resource, this function should return nil for searchParams.
	// In that case, the policy will deny access.
	relatedResourceSearchParams func(ctx context.Context, resource T) (resourceType string, searchParams *url.Values)
}

func (r RelatedResourceSearchPolicy[T, R]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	resourceType, searchParams := r.relatedResourceSearchParams(ctx, resource)
	if searchParams == nil {
		return false, nil
	}
	searchHandler := FHIRSearchOperationHandler[R]{
		fhirClient:  r.fhirClient,
		authzPolicy: r.relatedResourcePolicy,
	}
	results, _, err := searchHandler.searchAndFilter(ctx, *searchParams, &principal, resourceType)
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

var _ Policy[fhir.CarePlan] = &CareTeamMemberPolicy[fhir.CarePlan]{}

// CareTeamMemberPolicy is a policy that allows access if the user is a member of the care team.
type CareTeamMemberPolicy[T fhir.CarePlan] struct {
}

func (c CareTeamMemberPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	carePlan, ok := any(resource).(fhir.CarePlan)
	if !ok {
		return false, fmt.Errorf("resource is not a CarePlan")
	}
	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	// INT-630: We changed CareTeam to be contained within the CarePlan, but old test data in the CarePlan resource does not have CareTeam.
	//          For temporary backwards compatibility, ignore these CarePlans. It can be removed when old data has been purged.
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Unable to derive CareTeam from CarePlan, ignoring CarePlan for authorizing access to FHIR Patient resource (carePlanID=%s)", *carePlan.Id)
		return false, nil
	}
	return validatePrincipalInCareTeam(principal, careTeam) == nil, nil
	// TODO: Re-implement. We need this logic, but AuditEvent is not a suitable mechanism
	// For patients not yet authorized, check if the requester is the creator
	//for _, patient := range patients {
	//	// Skip if patient is already authorized through CareTeam
	//	if slices.ContainsFunc(retPatients, func(p fhir.Patient) bool {
	//		return *p.Id == *patient.Id
	//	}) {
	//		continue
	//	}
	//
	//	// Check if requester is the creator
	//	isCreator, err := s.isCreatorOfResource(ctx, principal, "Patient", *patient.Id)
	//	if err == nil && isCreator {
	//		log.Ctx(ctx).Debug().Msgf("User is creator of Patient/%s", *patient.Id)
	//		retPatients = append(retPatients, patient)
	//	}
	//}
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
