//go:generate mockgen -destination=./authz_mock.go -package=careplanservice -source authz.go
package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
)

type PolicyDecision struct {
	Allowed bool
	Reasons []string
}

type Policy[T any] interface {
	HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error)
}

var _ Policy[any] = &AnyMatchPolicy[any]{}

// AnyMatchPolicy is a policy that allows access if any of the policies in the list allow access.
type AnyMatchPolicy[T any] struct {
	Policies []Policy[T]
}

func (e AnyMatchPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	for _, policy := range e.Policies {
		decision, err := policy.HasAccess(ctx, resource, principal)
		if err != nil {
			return nil, err
		}
		if decision.Allowed {
			return &PolicyDecision{
				Allowed: true,
				Reasons: append([]string{"AnyMatchPolicy"}, decision.Reasons...),
			}, nil
		}
	}
	return &PolicyDecision{Allowed: false, Reasons: []string{"AnyMatchPolicy: none match"}}, nil
}

var _ Policy[any] = &LocalOrganizationPolicy[any]{}

// LocalOrganizationPolicy is a policy that allows access if the principal is a local organization.
type LocalOrganizationPolicy[T any] struct {
	profile profile.Provider
}

func (l LocalOrganizationPolicy[T]) HasAccess(ctx context.Context, _ T, principal auth.Principal) (*PolicyDecision, error) {
	localIdentities, err := l.profile.Identities(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get local identities: %w", err)
	}
	for _, localIdentity := range localIdentities {
		for _, requesterIdentifier := range principal.Organization.Identifier {
			for _, localIdentifier := range localIdentity.Identifier {
				if coolfhir.IdentifierEquals(&localIdentifier, &requesterIdentifier) {
					return &PolicyDecision{
						Allowed: true,
						Reasons: []string{"LocalOrganizationPolicy: principal is local organization"},
					}, nil
				}
			}
		}
	}
	return &PolicyDecision{
		Allowed: false,
		Reasons: []string{"LocalOrganizationPolicy: principal is not a local organization"},
	}, nil
}

var _ Policy[any] = &RelatedResourcePolicy[any, any]{}

// RelatedResourcePolicy is a policy that allows access if the user has access to the related resource(s).
// For instance, if the user has access to a ServiceRequest, if the user has access to the related Task.
type RelatedResourcePolicy[T any, R any] struct {
	fhirClient            fhirclient.Client
	relatedResourcePolicy Policy[R]
	// relatedResourceSearchParams is a function that returns the search parameters for the related resource.
	// If the resource lacks a reference to the related resource, this function should return nil for searchParams.
	// In that case, the policy will deny access.
	relatedResourceSearchParams func(ctx context.Context, resource T) (resourceType string, searchParams url.Values)
}

func (r RelatedResourcePolicy[T, R]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	resourceType, searchParams := r.relatedResourceSearchParams(ctx, resource)
	if searchParams == nil {
		return &PolicyDecision{
			Allowed: false,
			Reasons: []string{"RelatedResourcePolicy: no related resource search parameters"},
		}, nil
	}
	searchHandler := FHIRSearchOperationHandler[R]{
		fhirClient:  r.fhirClient,
		authzPolicy: r.relatedResourcePolicy,
	}
	const maxIterations = 100
	for i := 0; i < maxIterations; i++ {
		results, searchSet, policyDecisions, err := searchHandler.searchAndFilter(ctx, searchParams, &principal, resourceType)
		if err != nil {
			return nil, fmt.Errorf("related resource search (related resource type=%s): %w", resourceType, err)
		}
		if len(results) > 0 {
			// found a related resource the user has access to, grant access
			return &PolicyDecision{
				Allowed: true,
				Reasons: append([]string{"RelatedResourcePolicy: access to related resource(s)"}, policyDecisions[0].Reasons...),
			}, nil
		}
		// Try next page of search results if there is one
		hasNext := false
		for _, link := range searchSet.Link {
			if link.Relation == "next" {
				nextURL, err := url.Parse(link.Url)
				if err != nil {
					return nil, fmt.Errorf("invalid 'next' link for search set: %w", err)
				}
				searchParams = nextURL.Query()
				hasNext = true
			}
		}
		if !hasNext {
			break
		}
		// Make sure we don't loop endlessly due to a bug in ORCA or the FHIR server
		if i == maxIterations-1 {
			return nil, fmt.Errorf("max. search iterations reached (%d), possible bug", maxIterations)
		}
	}
	return &PolicyDecision{
		Allowed: false,
		Reasons: []string{"RelatedResourcePolicy: no access to related resource(s)"},
	}, nil
}

var _ Policy[fhir.Task] = &TaskOwnerOrRequesterPolicy[fhir.Task]{}

// TaskOwnerOrRequesterPolicy is a policy that allows access if the user is the owner of the task or the requester of the task.
type TaskOwnerOrRequesterPolicy[T fhir.Task] struct {
}

func (t TaskOwnerOrRequesterPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	resourceAsTask, ok := any(resource).(fhir.Task)
	if !ok {
		return nil, fmt.Errorf("resource is not a Task")
	}
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(&resourceAsTask, principal.Organization.Identifier)
	if isOwner {
		return &PolicyDecision{
			Allowed: true,
			Reasons: []string{"TaskOwnerOrRequesterPolicy: principal is Task owner"},
		}, nil
	}
	if isRequester {
		return &PolicyDecision{
			Allowed: true,
			Reasons: []string{"TaskOwnerOrRequesterPolicy: principal is Task requester"},
		}, nil
	}
	return &PolicyDecision{
		Allowed: false,
		Reasons: []string{"TaskOwnerOrRequesterPolicy: principal is neither Task owner or requester"},
	}, nil
}

var _ Policy[fhir.CarePlan] = &CareTeamMemberPolicy[fhir.CarePlan]{}

// CareTeamMemberPolicy is a policy that allows access if the user is a member of the care team.
type CareTeamMemberPolicy[T fhir.CarePlan] struct {
}

func (c CareTeamMemberPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	carePlan, ok := any(resource).(fhir.CarePlan)
	if !ok {
		return nil, fmt.Errorf("resource is not a CarePlan")
	}
	careTeam, err := coolfhir.CareTeamFromCarePlan(&carePlan)
	// INT-630: We changed CareTeam to be contained within the CarePlan, but old test data in the CarePlan resource does not have CareTeam.
	//          For temporary backwards compatibility, ignore these CarePlans. It can be removed when old data has been purged.
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Unable to derive CareTeam from CarePlan, ignoring CarePlan for authorizing access to FHIR Patient resource (carePlanID=%s)", *carePlan.Id)
		return &PolicyDecision{
			Allowed: false,
			Reasons: []string{"CareTeamMemberPolicy: unable to derive CareTeam from CarePlan"},
		}, nil
	}
	if validatePrincipalInCareTeam(principal, careTeam) == nil {
		return &PolicyDecision{
			Allowed: true,
			Reasons: []string{"CareTeamMemberPolicy: principal is member of CareTeam"},
		}, nil
	}
	return &PolicyDecision{
		Allowed: false,
		Reasons: []string{"CareTeamMemberPolicy: principal is not a member of CareTeam"},
	}, nil
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

// AnyonePolicy is a policy that allows access to anyone.
type AnyonePolicy[T any] struct {
}

func (e AnyonePolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	return &PolicyDecision{
		Allowed: true,
		Reasons: []string{"AnyonePolicy: anyone has access"},
	}, nil
}

var _ Policy[any] = &AnyonePolicy[any]{}

// CreatorPolicy is a policy that allows access if the principal is the creator of the resource.
type CreatorPolicy[T any] struct {
}

func (o CreatorPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	// TODO: Find a more suitable way to handle this auth.
	// The AuditEvent implementation has proven unsuitable and we are using the AuditEvent for unintended purposes.
	// For now, we can return true, as this will follow the same logic as was present before implementing the AuditEvent.

	return &PolicyDecision{
		Allowed: true,
		Reasons: []string{"CreatorPolicy: not implemented, anyone has access"},
	}, nil

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
