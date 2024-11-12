package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type BundleBuilder fhir.Bundle

func Transaction() *BundleBuilder {
	return &BundleBuilder{
		Type: fhir.BundleTypeTransaction,
	}
}

func SearchSet() *BundleBuilder {
	return &BundleBuilder{
		Type: fhir.BundleTypeSearchset,
	}
}

func (t *BundleBuilder) Create(resource interface{}, opts ...BundleEntryOption) *BundleBuilder {
	return t.Append(resource, &fhir.BundleEntryRequest{
		Method: fhir.HTTPVerbPOST,
		Url:    ResourceType(resource),
	}, nil, opts...)
}

func (t *BundleBuilder) Update(resource interface{}, path string, opts ...BundleEntryOption) *BundleBuilder {
	return t.Append(resource, &fhir.BundleEntryRequest{
		Method: fhir.HTTPVerbPUT,
		Url:    ResourceType(resource),
	}, nil, opts...)
}

func (t *BundleBuilder) Append(resource interface{}, request *fhir.BundleEntryRequest, response *fhir.BundleEntryResponse, opts ...BundleEntryOption) *BundleBuilder {
	data, err := json.Marshal(resource)
	if err != nil {
		return t
	}
	entry := fhir.BundleEntry{
		Resource: data,
		Request:  request,
		Response: response,
	}
	for _, opt := range opts {
		opt(&entry)
	}
	return t.AppendEntry(entry)
}

func (t *BundleBuilder) AppendEntry(entry fhir.BundleEntry) *BundleBuilder {
	t.Entry = append(t.Entry, entry)
	return t
}

func (t *BundleBuilder) Bundle() fhir.Bundle {
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

// ExecuteTransaction performs a FHIR transaction and returns the result bundle.
func ExecuteTransaction(fhirClient fhirclient.Client, bundle fhir.Bundle) (fhir.Bundle, error) {
	// Perform the FHIR transaction by creating the bundle
	var resultBundle fhir.Bundle
	if err := fhirClient.Create(bundle, &resultBundle, fhirclient.AtPath("/")); err != nil {
		return fhir.Bundle{}, fmt.Errorf("failed to execute FHIR transaction: %w", err)
	}

	if resultBundle.Entry == nil {
		return fhir.Bundle{}, fmt.Errorf("result bundle is nil")
	}

	log.Debug().Msgf("Executed Bundle successfully, got %d entries", len(resultBundle.Entry))
	// The transaction was successfully executed, return the result bundle
	return resultBundle, nil
}

func FetchBundleEntry(fhirClient fhirclient.Client, bundle *fhir.Bundle, filter func(i int, entry fhir.BundleEntry) bool, result interface{}) (*fhir.BundleEntry, error) {
	for i, currentEntry := range bundle.Entry {
		if currentEntry.Response == nil || currentEntry.Response.Location == nil {
			log.Error().Msg("entry.Response or entry.Response.Location is nil")
			continue
		}
		if !filter(i, currentEntry) {
			continue
		}
		headers := new(fhirclient.Headers)
		var responseData []byte
		if err := fhirClient.Read(*currentEntry.Response.Location, &responseData, fhirclient.ResponseHeaders(headers)); err != nil {
			return nil, errors.Join(ErrEntryNotFound, fmt.Errorf("failed to retrieve result Bundle entry (resource=%s): %w", *currentEntry.Response.Location, err))
		}
		if result != nil {
			if err := json.Unmarshal(responseData, result); err != nil {
				return nil, fmt.Errorf("unmarshal Bundle entry (target=%T): %w", result, err)
			}
		}
		response := currentEntry
		response.Resource = responseData
		return &response, nil
	}
	return nil, ErrEntryNotFound
}
