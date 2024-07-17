package coolfhir

import (
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"strings"
)

type TransactionBuilder fhir.Bundle

func Transaction() *TransactionBuilder {
	return &TransactionBuilder{
		Type: fhir.BundleTypeTransaction,
	}
}

func (t *TransactionBuilder) Create(resource interface{}, opts ...BundleEntryOption) *TransactionBuilder {
	return t.addEntry(resource, ResourceType(resource), fhir.HTTPVerbPOST, opts...)
}

func (t *TransactionBuilder) Update(resource interface{}, path string, opts ...BundleEntryOption) *TransactionBuilder {
	return t.addEntry(resource, path, fhir.HTTPVerbPUT, opts...)
}

func (t *TransactionBuilder) addEntry(resource interface{}, path string, verb fhir.HTTPVerb, opts ...BundleEntryOption) *TransactionBuilder {
	data, err := json.Marshal(resource)
	if err != nil {
		return t
	}
	entry := fhir.BundleEntry{
		Resource: data,
		Request: &fhir.BundleEntryRequest{
			Method: verb,
			Url:    path,
		},
	}
	for _, opt := range opts {
		opt(&entry)
	}
	t.Entry = append(t.Entry, entry)
	return t
}

func (t *TransactionBuilder) Bundle() fhir.Bundle {
	return fhir.Bundle(*t)
}

type BundleEntryOption func(entry *fhir.BundleEntry)

func WithFullUrl(fullUrl string) BundleEntryOption {
	return func(entry *fhir.BundleEntry) {
		entry.FullUrl = to.Ptr(fullUrl)
	}
}

type Resource struct {
	Type string `json:"resourceType"`
	ID   string `json:"id"`
}

func EntryIsOfType(resourceType string) func(entry fhir.BundleEntry) bool {
	return FilterResource(func(res Resource) bool {
		return res.Type == resourceType
	})
}

func EntryHasID(id string) func(entry fhir.BundleEntry) bool {
	return FilterResource(func(res Resource) bool {
		id = strings.TrimPrefix(id, res.Type+"/")
		return res.ID == id
	})
}

// FilterResource returns a filter function that filters resources in a bundle.
func FilterResource(fn func(resource Resource) bool) func(entry fhir.BundleEntry) bool {
	return func(entry fhir.BundleEntry) bool {
		var res Resource
		if err := json.Unmarshal(entry.Resource, &res); err != nil {
			return false
		}
		return fn(res)
	}
}

// FirstBundleEntry returns the entry in the bundle that matches the filter.
func FirstBundleEntry(bundle *fhir.Bundle, filter func(entry fhir.BundleEntry) bool) *fhir.BundleEntry {
	for _, entry := range bundle.Entry {
		if filter(entry) {
			return &entry
		}
	}
	return nil
}

// ResourcesInBundle unmarshals all entries in the bundle that match the given filter into the result.
func ResourcesInBundle(bundle *fhir.Bundle, filter func(entry fhir.BundleEntry) bool, result interface{}) error {
	var resources []json.RawMessage
	for _, entry := range bundle.Entry {
		if filter(entry) {
			resources = append(resources, entry.Resource)
		}
	}
	data, _ := json.Marshal(resources)
	return json.Unmarshal(data, result)
}

// ResourceInBundle unmarshals the entry in the bundle that matches the given filter into the result.
// If the entry is not found, ErrEntryNotFound is returned.
func ResourceInBundle(bundle *fhir.Bundle, filter func(entry fhir.BundleEntry) bool, result interface{}) error {
	resourceType := ResourceType(result)
	if resourceType == "" {
		return fmt.Errorf("can't infer resouce type from %T", result)
	}
	for _, entry := range bundle.Entry {
		var resource Resource
		if json.Unmarshal(entry.Resource, &resource) != nil {
			continue
		}
		if filter(entry) && resource.Type == resourceType {
			if err := json.Unmarshal(entry.Resource, result); err != nil {
				return fmt.Errorf("unmarshal Bundle entry (target=%T): %w", result, err)
			}
			return nil
		}
	}
	return ErrEntryNotFound
}
