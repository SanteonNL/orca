package test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"strings"
)

type BaseResource struct {
	Id         string            `json:"id"`
	Identifier []fhir.Identifier `json:"identifier"`
	Type       string            `json:"resourceType"`
	Data       []byte            `json:"-"`
}

var _ fhirclient.Client = &StubFHIRClient{}

type StubFHIRClient struct {
	Resources []interface{}
}

func (s StubFHIRClient) Read(path string, target any, opts ...fhirclient.Option) error {
	return s.ReadWithContext(context.Background(), path, target, opts...)
}

func (s StubFHIRClient) ReadWithContext(ctx context.Context, path string, target any, opts ...fhirclient.Option) error {
	for _, resource := range s.Resources {
		var baseResource BaseResource
		unmarshalInto(resource, &baseResource)
		if path == baseResource.Type+"/"+baseResource.Id {
			if err := json.Unmarshal(baseResource.Data, target); err != nil {
				panic(err)
			}
			return nil
		}
	}
	return errors.New("resource not found")
}

func (s StubFHIRClient) Create(resource any, result any, opts ...fhirclient.Option) error {
	resourceType := coolfhir.ResourceType(resource)
	if resourceType == "" {
		return fmt.Errorf("can't defer resource type of %T", resource)
	}
	var resourceAsMap map[string]interface{}
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
	unmarshalInto(resource, result)
	return nil
}

func (s StubFHIRClient) Search(resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	return s.SearchWithContext(context.Background(), resourceType, query, target, opts...)
}

func (s StubFHIRClient) SearchWithContext(ctx context.Context, resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	var candidates []BaseResource
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
		default:
			return fmt.Errorf("unsupported query parameter: %s", name)
		}
	}

	result := fhir.Bundle{
		Type:  fhir.BundleTypeSearchset,
		Total: to.Ptr(len(candidates)),
	}
	for _, candidate := range candidates {
		result.Entry = append(result.Entry, fhir.BundleEntry{
			Resource: candidate.Data,
		})
	}
	resultJSON, _ := json.Marshal(result)
	return json.Unmarshal(resultJSON, target)
}

func (s StubFHIRClient) CreateWithContext(ctx context.Context, resource any, result any, opts ...fhirclient.Option) error {
	panic("implement me")
}

func (s StubFHIRClient) Update(path string, resource any, result any, opts ...fhirclient.Option) error {
	panic("implement me")
}

func (s StubFHIRClient) UpdateWithContext(ctx context.Context, path string, resource any, result any, opts ...fhirclient.Option) error {
	panic("implement me")
}

func (s StubFHIRClient) Path(path ...string) *url.URL {
	panic("implement me")
}

func unmarshalInto(resource interface{}, target interface{}) {
	resJSON, err := json.Marshal(resource)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(resJSON, &target); err != nil {
		panic(err)
	}
	baseResource, ok := target.(*BaseResource)
	if ok {
		baseResource.Data = resJSON
	}
}
