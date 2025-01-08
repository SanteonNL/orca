package test

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"net/url"
)

var _ fhirclient.Client = &StubFHIRClient{}

type StubFHIRClient struct {
	Resources []interface{}
}

func (s StubFHIRClient) Read(path string, target any, opts ...fhirclient.Option) error {
	return s.ReadWithContext(context.Background(), path, target, opts...)
}

func (s StubFHIRClient) ReadWithContext(ctx context.Context, path string, target any, opts ...fhirclient.Option) error {
	type BaseResource struct {
		Id   string `json:"id"`
		Type string `json:"resourceType"`
	}
	for _, resource := range s.Resources {
		resJSON, err := json.Marshal(resource)
		if err != nil {
			panic(err)
		}
		var baseResource BaseResource
		if err := json.Unmarshal(resJSON, &baseResource); err != nil {
			panic(err)
		}
		if path == baseResource.Type+"/"+baseResource.Id {
			if err := json.Unmarshal(resJSON, target); err != nil {
				panic(err)
			}
			return nil
		}
	}
	return errors.New("resource not found")
}

func (s StubFHIRClient) Search(resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	panic("implement me")
}

func (s StubFHIRClient) SearchWithContext(ctx context.Context, resourceType string, query url.Values, target any, opts ...fhirclient.Option) error {
	panic("implement me")
}

func (s StubFHIRClient) Create(resource any, result any, opts ...fhirclient.Option) error {
	panic("implement me")
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
