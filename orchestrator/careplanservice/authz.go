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
	"reflect"
)

const CreatorExtensionURL = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/resource-creator"

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

var _ Policy[any] = &LocalOrganizationPolicy[any]{}

// LocalOrganizationPolicy is a policy that allows access if the principal is a local organization.
type LocalOrganizationPolicy[T any] struct {
	profile profile.Provider
}

func (l LocalOrganizationPolicy[T]) HasAccess(ctx context.Context, _ T, principal auth.Principal) (bool, error) {
	localIdentities, err := l.profile.Identities(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get local identities: %w", err)
	}
	for _, localIdentity := range localIdentities {
		for _, requesterIdentifier := range principal.Organization.Identifier {
			for _, localIdentifier := range localIdentity.Identifier {
				if coolfhir.IdentifierEquals(&localIdentifier, &requesterIdentifier) {
					return true, nil
				}
			}
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
	// relatedResourceSearchParams is a function that returns the search parameters for the related resource.
	// If the resource lacks a reference to the related resource, this function should return nil for searchParams.
	// In that case, the policy will deny access.
	relatedResourceSearchParams func(ctx context.Context, resource T) (resourceType string, searchParams *url.Values)
}

func (r RelatedResourcePolicy[T, R]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
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

// AnyonePolicy is a policy that allows access to anyone.
type AnyonePolicy[T any] struct {
}

func (e AnyonePolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	return true, nil
}

var _ Policy[any] = &AnyonePolicy[any]{}

// CreatorPolicy is a policy that allows access if the principal is the creator of the resource.
type CreatorPolicy[T any] struct {
}

func (o CreatorPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	// Check if the resource has a resource-creator extension matching the principal's organization identifier
	resourceValue := reflect.ValueOf(resource)
	if resourceValue.Kind() == reflect.Ptr {
		resourceValue = resourceValue.Elem()
	}
	extensionField := resourceValue.FieldByName("Extension")
	if !extensionField.IsValid() {
		// Resource doesn't have an Extension field
		return false, nil
	}
	extensions, ok := extensionField.Interface().([]fhir.Extension)
	if !ok {
		// Extension field is not of the expected type
		return false, nil
	}

	for _, extension := range extensions {
		if extension.Url == CreatorExtensionURL && extension.ValueReference != nil && extension.ValueReference.Identifier != nil {
			// Compare with principal's organization identifiers
			for _, orgIdentifier := range principal.Organization.Identifier {
				if coolfhir.IdentifierEquals(extension.ValueReference.Identifier, &orgIdentifier) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
