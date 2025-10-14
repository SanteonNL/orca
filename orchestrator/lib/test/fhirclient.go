package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/must"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type BaseResource struct {
	Id         string            `json:"id"`
	Identifier []fhir.Identifier `json:"identifier"`
	Type       string            `json:"resourceType"`
	Data       []byte            `json:"-"`
	URL        string            `json:"url"`
}

var _ fhirclient.Client = &StubFHIRClient{}

type StubFHIRClient struct {
	Resources []any
	Metadata  fhir.CapabilityStatement
	// CreatedResources is a list of resources that have been created using this client.
	// It's not used by the client itself, but can be used by tests to verify that the client has been used correctly.
	CreatedResources map[string][]any
	// Error is an error that will be returned by all methods of this client.
	Error error
}

func (s StubFHIRClient) Read(path string, target any, opts ...fhirclient.Option) error {
	return s.ReadWithContext(context.Background(), path, target, opts...)
}

func (s StubFHIRClient) ReadWithContext(ctx context.Context, path string, target any, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	if path == "metadata" {
		unmarshalInto(s.Metadata, &target)
		return nil
	}
	for _, resource := range s.Resources {
		var baseResource BaseResource
		unmarshalInto(resource, &baseResource)
		if path == baseResource.Type+"/"+baseResource.Id {
			switch target.(type) {
			case *[]byte:
				*target.(*[]byte) = baseResource.Data
			default:
				if err := json.Unmarshal(baseResource.Data, target); err != nil {
					panic(err)
				}
			}
			if err := processPostRequestOpts(opts); err != nil {
				return err
			}
			return nil
		}
	}

	return fhirclient.OperationOutcomeError{
		HttpStatusCode: http.StatusNotFound,
	}
}

func processPostRequestOpts(opts []fhirclient.Option) error {
	for _, opt := range opts {
		if post, ok := opt.(fhirclient.PostRequestOption); ok {
			if err := post(nil, &http.Response{
				Status:     "200 OK",
				StatusCode: 200,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s StubFHIRClient) Create(resource any, result any, opts ...fhirclient.Option) error {
	return s.CreateWithContext(context.Background(), resource, result, opts...)
}

func (s StubFHIRClient) Search(resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	return s.SearchWithContext(context.Background(), resourceType, query, target, opts...)
}

func (s *StubFHIRClient) SearchWithContext(ctx context.Context, resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	var candidates []BaseResource
	var additionalResources []BaseResource
	for _, res := range s.Resources {
		var baseResource BaseResource
		unmarshalInto(res, &baseResource)
		if baseResource.Type == resourceType {
			candidates = append(candidates, baseResource)
		}
	}

	filterCandidates := func(predicate func(BaseResource) bool) {
		var filtered []BaseResource
		for _, candidate := range candidates {
			if predicate(candidate) {
				filtered = append(filtered, candidate)
			}
		}
		candidates = filtered
	}

	count := 100
	startAt := 0
	for name, values := range query {
		if len(values) != 1 {
			return fmt.Errorf("multiple values for query parameter: %s", name)
		}
		value := values[0]
		switch name {
		case "identifier":
			token := strings.Split(value, "|")
			filterCandidates(func(candidate BaseResource) bool {
				for _, identifier := range candidate.Identifier {
					if (token[0] == "" || to.EmptyString(identifier.System) == token[0]) &&
						(token[1] == "" || to.EmptyString(identifier.Value) == token[1]) {
						return true
					}
				}
				return false
			})
		case "_id":
			filterCandidates(func(candidate BaseResource) bool {
				return candidate.Id == value
			})
		case "_include":
			filterCandidates(func(candidate BaseResource) bool {
				if candidate.Type == "CarePlan" {
					if value == "CarePlan:care-team" {
						for _, res := range s.Resources {
							careTeam, ok := res.(fhir.CareTeam)
							if !ok {
								continue
							}
							var carePlan fhir.CarePlan
							err := json.Unmarshal(candidate.Data, &carePlan)
							if err != nil {
								panic(err)
							}
							for _, reference := range carePlan.CareTeam {
								if *reference.Reference == "CareTeam/"+*careTeam.Id {
									var baseRes BaseResource
									unmarshalInto(res, &baseRes)
									additionalResources = append(additionalResources, baseRes)
									return true
								}
							}
						}
					}
				}
				return true
			})
		case "_revinclude":
			filterCandidates(func(candidate BaseResource) bool {
				if candidate.Type == "Task" {
					if value == "Task:part-of" {
						for _, res := range s.Resources {
							task, ok := res.(fhir.Task)
							if !ok {
								continue
							}
							if task.PartOf == nil {
								continue
							}
							for _, partOf := range task.PartOf {
								if *partOf.Reference == "Task/"+candidate.Id {
									var baseRes BaseResource
									unmarshalInto(res, &baseRes)
									additionalResources = append(additionalResources, baseRes)
									return true
								}
							}
						}
					}
				}
				return false
			})
		case "output-reference":
			filterCandidates(func(candidate BaseResource) bool {
				if candidate.Type != "Task" {
					return false
				}
				var task fhir.Task
				if err := json.Unmarshal(candidate.Data, &task); err != nil {
					panic(err)
				}
				for _, output := range task.Output {
					if output.ValueReference != nil &&
						output.ValueReference.Reference != nil &&
						*output.ValueReference.Reference == value {
						return true
					}
				}
				return false
			})
		case "focus":
			filterCandidates(func(candidate BaseResource) bool {
				if candidate.Type != "Task" {
					return false
				}
				var task fhir.Task
				if err := json.Unmarshal(candidate.Data, &task); err != nil {
					panic(err)
				}
				return task.Focus != nil && *task.Focus.Reference == value
			})
		case "subject":
			filterCandidates(func(candidate BaseResource) bool {
				if candidate.Type != "CarePlan" {
					return false
				}
				var carePlan fhir.CarePlan
				if err := json.Unmarshal(candidate.Data, &carePlan); err != nil {
					panic(err)
				}
				if carePlan.Subject.Reference != nil && *carePlan.Subject.Reference == value {
					return true
				}
				if carePlan.Subject.Identifier != nil {
					token := fmt.Sprintf("%s|%s", to.EmptyString(carePlan.Subject.Identifier.System), to.EmptyString(carePlan.Subject.Identifier.Value))
					if value == token {
						return true
					}
				}
				return false
			})
		case "url":
			filterCandidates(func(candidate BaseResource) bool {
				return candidate.URL == value
			})
		case "_start_at":
			// custom parameter for pagination using 'next' link
			var err error
			startAt, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid _start_at parameter value (must be an int): %s", value)
			}
			if startAt < 0 {
				return fmt.Errorf("invalid _start_at parameter value (must be >= 0): %s", value)
			}
		case "_count":
			var err error
			count, err = strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid _count parameter value: %s", value)
			}
		default:
			return fmt.Errorf("unsupported query parameter: %s", name)
		}
	}

	result := fhir.Bundle{
		Type: fhir.BundleTypeSearchset,
	}

	idxStart := startAt
	idxEnd := idxStart + count
	if idxStart > len(candidates) {
		candidates = []BaseResource{}
	} else {
		if idxEnd > len(candidates) {
			idxEnd = len(candidates)
		} else {
			nextURLQuery, _ := url.ParseQuery(query.Encode())
			nextURLQuery.Set("_start_at", strconv.Itoa(idxEnd))
			result.Link = append(result.Link, fhir.BundleLink{
				Relation: "next",
				Url:      "https://example.com/fhir/" + resourceType + "/_search?" + nextURLQuery.Encode(),
			})
		}
		candidates = candidates[idxStart:idxEnd]
	}

	for _, candidate := range candidates {
		result.Entry = append(result.Entry, fhir.BundleEntry{
			Resource: candidate.Data,
		})
	}
	for _, additionalResource := range additionalResources {
		result.Entry = append(result.Entry, fhir.BundleEntry{
			Resource: additionalResource.Data,
		})
	}
	unmarshalInto(result, target)
	if err := processPostRequestOpts(opts); err != nil {
		return err
	}
	return nil
}

func (s *StubFHIRClient) CreateWithContext(_ context.Context, resource any, result any, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}

	var baseResource BaseResource
	unmarshalInto(resource, &baseResource)
	resourceType := baseResource.Type

	if resourceType == "" {
		return fmt.Errorf("can't defer resource type of %T", resource)
	}
	var resourceAsMap = make(map[string]interface{})
	unmarshalInto(resource, resourceAsMap)
	if resourceAsMap["id"] == nil {
		resourceAsMap["id"] = uuid.NewString()
	} else {
		// Check if it doesn't already exist
		for _, existingResource := range s.Resources {
			var existingResourceBase BaseResource
			unmarshalInto(existingResource, &existingResourceBase)
			if resourceType == existingResourceBase.Type && existingResourceBase.Id == resourceAsMap["id"] {
				return errors.New("resource already exists")
			}
		}
	}
	s.Resources = append(s.Resources, resource)
	if s.CreatedResources == nil {
		s.CreatedResources = make(map[string][]any)
	}
	s.CreatedResources[resourceType] = append(s.CreatedResources[resourceType], resource)
	unmarshalInto(resource, result)
	return nil
}

func (s StubFHIRClient) Update(path string, resource any, result any, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	panic("implement me")
}

func (s *StubFHIRClient) UpdateWithContext(ctx context.Context, path string, resource any, result any, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	// Find and update the resource in our Resources slice
	// Handle paths like "Task/taskId" by extracting the resource type and ID
	if strings.HasPrefix(path, "Task/") {
		if updatedTask, ok := resource.(fhir.Task); ok && updatedTask.Id != nil {
			for i, existingResource := range s.Resources {
				if existingTask, ok := existingResource.(fhir.Task); ok {
					if existingTask.Id != nil && *existingTask.Id == *updatedTask.Id {
						s.Resources[i] = updatedTask
						// Copy the updated task to the result if it's provided
						if resultPtr, ok := result.(*fhir.Task); ok {
							*resultPtr = updatedTask
						}
						return nil
					}
				}
			}
		}
	}
	return nil
}

func (s StubFHIRClient) Delete(path string, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	panic("implement me")
}

func (s StubFHIRClient) DeleteWithContext(ctx context.Context, path string, opts ...fhirclient.Option) error {
	if s.Error != nil {
		return s.Error
	}
	panic("implement me")
}

func (s StubFHIRClient) Path(path ...string) *url.URL {
	return must.ParseURL("stub:" + strings.Join(path, "/"))
}

func unmarshalInto(resource interface{}, target interface{}) {
	resJSON, err := json.Marshal(resource)
	if err != nil {
		panic(err)
	}
	switch t := target.(type) {
	case *[]byte:
		*t = resJSON
	default:
		if err := json.Unmarshal(resJSON, &target); err != nil {
			panic(err)
		}
		baseResource, ok := target.(*BaseResource)
		if ok {
			baseResource.Data = resJSON
		}
	}
}
