package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Writes an OperationOutcome based on the given error as HTTP response.
func (s *Service) writeOperationOutcomeFromError(err error, desc string, httpResponse http.ResponseWriter) {
	log.Info().Msgf("%s failed: %v", desc, err)
	diagnostics := fmt.Sprintf("%s failed: %s", desc, err.Error())

	issue := fhir.OperationOutcomeIssue{
		Severity:    fhir.IssueSeverityError,
		Code:        fhir.IssueTypeProcessing,
		Diagnostics: to.Ptr(diagnostics),
	}

	outcome := fhir.OperationOutcome{
		Issue: []fhir.OperationOutcomeIssue{issue},
	}

	coolfhir.SendResponse(httpResponse, http.StatusBadRequest, outcome)
}

func (s *Service) getCarePlanAndCareTeams(carePlanReference string) (fhir.CarePlan, []fhir.CareTeam, *fhirclient.Headers, error) {
	bundle := fhir.Bundle{}
	var carePlan fhir.CarePlan
	var careTeams []fhir.CareTeam
	headers := new(fhirclient.Headers)

	carePlanId := strings.TrimPrefix(carePlanReference, "CarePlan/")

	err := s.fhirClient.Read("CarePlan", &bundle, fhirclient.QueryParam("_id", carePlanId), fhirclient.QueryParam("_include", "CarePlan:care-team"), fhirclient.ResponseHeaders(headers))
	if err != nil {
		return fhir.CarePlan{}, nil, nil, err
	}

	err = coolfhir.ResourceInBundle(&bundle, coolfhir.EntryIsOfType("CarePlan"), &carePlan)
	if err != nil {
		return fhir.CarePlan{}, nil, nil, err
	}

	err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
	if len(careTeams) == 0 {
		return fhir.CarePlan{}, nil, nil, &coolfhir.ErrorWithCode{
			Message:    "CareTeam not found in bundle",
			StatusCode: http.StatusNotFound,
		}
	}

	return carePlan, careTeams, headers, nil
}

// handleTaskBasedResourceAuth is a generic function to handle the authorization of a resource based on a Task
// it will check if the resource is based on a Task, and if so, fetch the Task and in doing so - validate the requester has access to the Task
// it returns an error if the Task is not found or the requester is not a participant of the CareTeam associated with the Task
func (s *Service) handleTaskBasedResourceAuth(ctx context.Context, headers *fhirclient.Headers, basedOn []fhir.Reference, resourceType string) error {
	if len(basedOn) != 1 {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("%s has invalid number of BasedOn values", resourceType),
			StatusCode: http.StatusInternalServerError,
		}
	}
	if basedOn[0].Reference == nil {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("%s has invalid BasedOn Reference", resourceType),
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Fetch the task this questionnaireResponse is based on
	// As long as we get a task back, we can assume the user has access to the service request
	if !strings.HasPrefix(*basedOn[0].Reference, "Task/") {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("%s BasedOn is not a Task", resourceType),
			StatusCode: http.StatusInternalServerError,
		}
	}
	taskId := strings.TrimPrefix(*basedOn[0].Reference, "Task/")
	_, err := s.handleGetTask(ctx, taskId, headers)
	if err != nil {
		return err
	}
	return nil
}

// handleMetaBasedUpdateResourceAuth is a generic function to handle the authorization of a resource based on the Meta.Source
// it will check if the requester is the creator of the resource as per the Meta.Source
// it returns an error if the requester is not the creator of the resource
func (s *Service) handleMetaBasedUpdateResourceAuth(existingResourceMeta *fhir.Meta, updatedResourceMeta *fhir.Meta, principal auth.Principal, resourceType string) error {
	if existingResourceMeta == nil || updatedResourceMeta == nil || existingResourceMeta.Source == nil || updatedResourceMeta.Source == nil {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("%s does not have Meta.Source defined", resourceType),
			StatusCode: http.StatusInternalServerError,
		}
	}

	// Validate that the Meta.Source is not being changed
	if *existingResourceMeta.Source != *updatedResourceMeta.Source {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("%s Meta.Source cannot be changed", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	// Validate that the requester is the creator of the resource as per the meta
	isCreator := false
	for _, org := range principal.Organization.Identifier {
		// TODO: Validate if this is an acceptable way to handle this
		// The Meta.Source may have a suffix hash applied, verify that the principal org matches up to the start of the hash
		if strings.HasPrefix(*existingResourceMeta.Source, fmt.Sprintf("%s/%s#", *org.System, *org.Value)) || *existingResourceMeta.Source == fmt.Sprintf("%s/%s", *org.System, *org.Value) {
			isCreator = true
			break
		}
	}
	if !isCreator {
		return &coolfhir.ErrorWithCode{
			Message:    fmt.Sprintf("requester does not have access to update %s", resourceType),
			StatusCode: http.StatusForbidden,
		}
	}

	return nil
}

// handleMetaBasedResourceCreate is a generic function to handle the update of a resource based on the Meta.Source
// when the resource is created we set Meta.source to the organization that created it
func handleMetaBasedResourceUpdate[T any](s *Service, resourceType string, ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder, fieldValidationFunc func(*T, *T) error) (FHIRHandlerResult, error) {
	log.Info().Msgf("Updating %s: %s", resourceType, request.RequestUrl)
	var resource T
	err := json.Unmarshal(request.ResourceData, &resource)
	if err != nil {
		return nil, fmt.Errorf("invalid %T: %w", resource, err)
	}

	// Easier way to get ID, Meta than through reflection
	type ResourceRequiredFields struct {
		Id   *string    `json:"id,omitempty"`
		Meta *fhir.Meta `json:"meta,omitempty"`
	}
	var resourceRequiredFields ResourceRequiredFields
	err = json.Unmarshal(request.ResourceData, &resourceRequiredFields)
	if err != nil {
		return nil, fmt.Errorf("invalid %T: %w", resource, err)
	}

	var resourceExisting T
	// TODO: Handle PUT when the resource does not exist, this has different auth requirements
	if request.ResourceId == "" {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "missing ID in request",
			StatusCode: http.StatusBadRequest,
		}
	} else {
		if (resourceRequiredFields.Id != nil && request.ResourceId != "") && request.ResourceId != *resourceRequiredFields.Id || request.ResourcePath != resourceType+"/"+request.ResourceId {
			return nil, coolfhir.BadRequestError("ID in request URL does not match ID in resource")
		}
		err = s.fhirClient.Read(resourceType+"/"+request.ResourceId, &resourceExisting)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", resourceType, err)
	}

	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get principal from context: %w", err)
	}
	if resourceRequiredFields.Meta == nil {
		return nil, fmt.Errorf("cannot determine creator of %s", resourceType)
	}

	// Marshall existing type to JSON, then marshall into required fields struct
	var existingResourceRequiredFields ResourceRequiredFields
	existingResourceJSON, err := json.Marshal(resourceExisting)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal existing resource: %w", err)
	}
	err = json.Unmarshal(existingResourceJSON, &existingResourceRequiredFields)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal existing resource: %w", err)
	}

	err = s.handleMetaBasedUpdateResourceAuth(
		existingResourceRequiredFields.Meta,
		resourceRequiredFields.Meta,
		principal,
		resourceType,
	)
	if err != nil {
		return nil, err
	}

	// Validate the resource fields
	err = fieldValidationFunc(&resource, &resourceExisting)
	if err != nil {
		return nil, err
	}

	resourceBundleEntry := request.bundleEntryWithResource(resource)
	tx = tx.AppendEntry(resourceBundleEntry)
	idx := len(tx.Entry) - 1

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		var updatedResource T
		result, err := coolfhir.NormalizeTransactionBundleResponseEntry(s.fhirClient, s.fhirURL, &resourceBundleEntry, &txResult.Entry[idx], &updatedResource)
		if errors.Is(err, coolfhir.ErrEntryNotFound) {
			// Bundle execution succeeded, but could not read result entry.
			updatedResource = resource
		} else if err != nil {
			return nil, nil, err
		}
		return result, nil, nil
	}, nil
}

// handleSearchResource is a generic function to search for a resource of a given type and return the results
// it returns a processed list of the required resource type, the full bundle and an error
func handleSearchResource[T any](s *Service, resourceType string, queryParams url.Values, headers *fhirclient.Headers) ([]T, *fhir.Bundle, error) {
	params := []fhirclient.Option{}
	for k, v := range queryParams {
		for _, value := range v {
			params = append(params, fhirclient.QueryParam(k, value))
		}
	}

	params = append(params, fhirclient.ResponseHeaders(headers))
	var bundle fhir.Bundle
	err := s.fhirClient.Read(resourceType, &bundle, params...)
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

func validatePrincipalInCareTeams(principal auth.Principal, careTeams []fhir.CareTeam) error {
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeams, principal.Organization.Identifier)
	if participant == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Participant is not part of CareTeam",
			StatusCode: http.StatusForbidden,
		}
	}
	return nil
}

// matchResourceIDs matches whether the ID in the request URL matches the ID in the resource.
// This is important for PUT requests, where the ID in the URL is the ID of the resource to update.
// They do not need to be set both, but if they are, they should match.
func matchResourceIDs(request *FHIRHandlerRequest, idFromResource *string) error {
	if (idFromResource != nil && request.ResourceId != "") && request.ResourceId != *idFromResource {
		return &coolfhir.ErrorWithCode{
			Message:    "ID in request URL does not match ID in resource",
			StatusCode: http.StatusBadRequest,
		}
	}
	return nil
}

// filterMatchingResourcesInBundle will find all resources in the bundle of the given type with a matching ID and return a new bundle with only those resources
func filterMatchingResourcesInBundle(bundle *fhir.Bundle, resourceTypes []string, references []string) fhir.Bundle {
	newBundle := fhir.Bundle{
		Entry: []fhir.BundleEntry{},
	}

	operationOutcomeErrors := []fhir.BundleEntry{}
	for i, entry := range bundle.Entry {
		var resourceInBundle coolfhir.Resource
		err := json.Unmarshal(entry.Resource, &resourceInBundle)
		if err != nil {
			// We don't want to fail the whole operation if one resource fails to unmarshal
			log.Error().Msgf("filterMatchingResourcesInBundle: Failed to unmarshal resource: %v", err)
			operationOutcomeEntry, err := coolfhir.CreateOperationOutcomeBundleEntryFromError(err, "Failed to unmarshal resource")
			if err != nil {
				log.Error().Msgf("filterMatchingResourcesInBundle: Failed to marshal operation outcome: %v", err)
				continue
			}
			operationOutcomeErrors = append(operationOutcomeErrors, *operationOutcomeEntry)
			continue
		}

		if slices.Contains(resourceTypes, resourceInBundle.Type) {
			for _, ref := range references {
				parts := strings.Split(ref, "/")
				if len(parts) != 2 {
					log.Error().Msgf("filterMatchingResourcesInBundle: Invalid reference format: %s", ref)
					operationOutcomeEntry, err := coolfhir.CreateOperationOutcomeBundleEntryFromError(fmt.Errorf("Invalid reference format: %s", ref), "Invalid reference format")
					if err != nil {
						log.Error().Msgf("filterMatchingResourcesInBundle: Failed to marshal operation outcome: %v", err)
						continue
					}
					operationOutcomeErrors = append(operationOutcomeErrors, *operationOutcomeEntry)
					continue
				}
				if parts[0] == resourceInBundle.Type && parts[1] == resourceInBundle.ID {
					newBundle.Entry = append(newBundle.Entry, bundle.Entry[i])
					break
				}
			}
		}
	}
	newBundle.Entry = append(newBundle.Entry, operationOutcomeErrors...)

	return newBundle
}
