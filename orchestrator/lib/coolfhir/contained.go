package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var (
	ErrCareTeamNotFound = errors.New("CareTeam not found in FHIR Bundle")
	ErrInvalidReference = errors.New("invalid reference, expecting local reference")
)

type ContainedResource struct {
	Resource
	Raw json.RawMessage `json:"-"`
}

func (r *ContainedResource) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.Resource); err != nil {
		return err
	}

	r.Raw = data

	return nil
}

func (r *ContainedResource) MarshalJSON() ([]byte, error) {
	return r.Raw, nil
}

func UpdateContainedResource(contained json.RawMessage, id *fhir.Reference, resourceType string, resource any) (json.RawMessage, error) {
	if !IsLocalReference(id) {
		return nil, ErrInvalidReference
	}

	data, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var resources []ContainedResource

	if err := json.Unmarshal(contained, &resources); err != nil {
		return nil, fmt.Errorf("failed to unmarshal contained resources: %w", err)
	}

	ref := *id.Reference

	for i, containedResource := range resources {
		if containedResource.ID == ref[1:] && containedResource.Type == resourceType {
			containedResource.Raw = data
			resources[i] = containedResource

			return json.Marshal(resources)
		}
	}

	return nil, fmt.Errorf("contained resource not found")
}

func ResolveCareTeam(carePlan *fhir.CarePlan) (*fhir.CareTeam, error) {
	if len(carePlan.CareTeam) != 1 {
		return nil, errors.New("CarePlan must have exactly one CareTeam")
	}

	var resources []ContainedResource

	if len(carePlan.Contained) > 0 {
		if err := json.Unmarshal(carePlan.Contained, &resources); err != nil {
			return nil, fmt.Errorf("failed to unmarshal contained resources: %w", err)
		}
	}

	careTeamRef := carePlan.CareTeam[0]

	if !IsLocalReference(&careTeamRef) {
		return nil, fmt.Errorf("invalid reference: %s", *careTeamRef.Reference)
	}

	for _, resource := range resources {
		if resource.ID != (*careTeamRef.Reference)[1:] {
			continue
		}

		careTeam, err := fhir.UnmarshalCareTeam(resource.Raw)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal careteam: %w", err)
		}

		return &careTeam, nil
	}

	return nil, ErrCareTeamNotFound
}
