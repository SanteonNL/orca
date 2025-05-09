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

const CreatorExtensionURL = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/resource-creator"

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
	relatedResourceSearchParams func(ctx context.Context, resource T) (resourceType string, searchParams *url.Values)
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
	results, _, policyDecisions, err := searchHandler.searchAndFilter(ctx, *searchParams, &principal, resourceType)
	if err != nil {
		return nil, fmt.Errorf("related resource search (related resource type=%s): %w", resourceType, err)
	}
	if len(results) > 0 {
		return &PolicyDecision{
			Allowed: true,
			Reasons: append([]string{"RelatedResourcePolicy: access to related resource(s)"}, policyDecisions[0].Reasons...),
		}, nil
	} else {
		return &PolicyDecision{
			Allowed: false,
			Reasons: []string{"RelatedResourcePolicy: no access to related resource(s)"},
		}, nil
	}

}

var _ Policy[*fhir.Task] = &TaskOwnerOrRequesterPolicy[fhir.Task]{}

// TaskOwnerOrRequesterPolicy is a policy that allows access if the user is the owner of the task or the requester of the task.
type TaskOwnerOrRequesterPolicy[T fhir.Task] struct {
}

func (t TaskOwnerOrRequesterPolicy[T]) HasAccess(ctx context.Context, resource *T, principal auth.Principal) (*PolicyDecision, error) {
	resourceAsTask, ok := any(resource).(*fhir.Task)
	if !ok {
		return nil, fmt.Errorf("resource is not a Task")
	}
	isOwner, isRequester := coolfhir.IsIdentifierTaskOwnerAndRequester(resourceAsTask, principal.Organization.Identifier)
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

var _ Policy[*fhir.CarePlan] = &CareTeamMemberPolicy[fhir.CarePlan]{}

// CareTeamMemberPolicy is a policy that allows access if the user is a member of the care team.
type CareTeamMemberPolicy[T fhir.CarePlan] struct {
}

func (c CareTeamMemberPolicy[T]) HasAccess(ctx context.Context, resource *T, principal auth.Principal) (*PolicyDecision, error) {
	carePlan, ok := any(resource).(*fhir.CarePlan)
	if !ok {
		return nil, fmt.Errorf("resource is not a CarePlan")
	}
	careTeam, err := coolfhir.CareTeamFromCarePlan(carePlan)
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
}

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
type CreatorPolicy[T fhir.HasExtension] struct {
}

func (o CreatorPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (*PolicyDecision, error) {
	for _, extension := range resource.GetExtension() {
		if extension.Url == CreatorExtensionURL && extension.ValueReference != nil && extension.ValueReference.Identifier != nil {
			// Compare with principal's organization identifiers
			for _, orgIdentifier := range principal.Organization.Identifier {
				if coolfhir.IdentifierEquals(extension.ValueReference.Identifier, &orgIdentifier) {
					return &PolicyDecision{
						Allowed: true,
						Reasons: []string{"CreatorPolicy: principal is the creator"},
					}, nil
				}
			}
		}
	}

	return &PolicyDecision{
		Allowed: false,
		Reasons: []string{"CreatorPolicy: principal is not the creator"},
	}, nil
}

var _ Policy[fhir.HasExtension] = &CreatorPolicy[fhir.HasExtension]{}
