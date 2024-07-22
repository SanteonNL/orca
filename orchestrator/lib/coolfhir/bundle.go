package coolfhir

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

type TransactionBuilder fhir.Bundle

func Transaction() *TransactionBuilder {
	return &TransactionBuilder{
		Type: fhir.BundleTypeTransaction,
	}
}

func (t *TransactionBuilder) Create(resource interface{}, opts ...BundleEntryOption) *TransactionBuilder {
	data, err := json.Marshal(resource)
	if err != nil {
		return t
	}
	entry := fhir.BundleEntry{
		Resource: data,
		Request: &fhir.BundleEntryRequest{
			Method: fhir.HTTPVerbPOST,
			Url:    ResourceType(resource),
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

func EntryIsOfType(resourceType string) func(entry fhir.BundleEntry) bool {
	return func(entry fhir.BundleEntry) bool {
		type Resource struct {
			Type string `json:"resourceType"`
		}
		var res Resource
		if err := json.Unmarshal(entry.Resource, &res); err != nil {
			return false
		}
		return res.Type == resourceType
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
