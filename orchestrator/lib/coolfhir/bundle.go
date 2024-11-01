package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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
	return t.Append(entry)
}

func (t *TransactionBuilder) Append(entry fhir.BundleEntry) *TransactionBuilder {
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

// ExecuteTransactionAndRespondWithEntry executes a transaction (Bundle) on the FHIR server and responds with the entry that matches the filter.
// It unmarshals the filtered entry into the given resultResource
func ExecuteTransactionAndRespondWithEntry(fhirClient fhirclient.Client, bundle fhir.Bundle, filter func(entry fhir.BundleEntry) bool, httpResponse http.ResponseWriter, resultResource any) error {
	return executeTransactionAndRespond(fhirClient, bundle, filter, httpResponse, resultResource, true)
}

// ExecuteTransactionAndRespondWithResultBundle executes a transaction (Bundle) on the FHIR server and responds with the result Bundle
// It unmarshals the filtered entry into the given resultResource
func ExecuteTransactionAndRespondWithResultBundle(fhirClient fhirclient.Client, bundle fhir.Bundle, filter func(entry fhir.BundleEntry) bool, httpResponse http.ResponseWriter, resultResource any) error {
	return executeTransactionAndRespond(fhirClient, bundle, filter, httpResponse, resultResource, false)
}

// executeTransactionAndRespond is a helper function that executes a transaction (Bundle) on the FHIR server and responds with either the filtered entry or the whole bundle
func executeTransactionAndRespond(fhirClient fhirclient.Client, bundle fhir.Bundle, filter func(entry fhir.BundleEntry) bool, httpResponse http.ResponseWriter, resultResource any, respondWithEntry bool) error {
	resultBundle, err := ExecuteTransaction(fhirClient, bundle)
	if err != nil {
		return err
	}

	log.Trace().Msgf("Found %d entries in result Bundle", len(resultBundle.Entry))

	for _, entry := range resultBundle.Entry {
		if entry.Response == nil || entry.Response.Location == nil {
			log.Error().Msg("entry.Response or entry.Response.Location is nil")
			continue
		}

		log.Trace().Msgf("entry.Response.Location found: %s", *entry.Response.Location)

		if !filter(entry) {
			continue
		}

		log.Trace().Msgf("filter matched on %s", *entry.Response.Location)

		// Read the resource from the FHIR server, to return it to the client.
		if resultResource == nil {
			// caller doesn't care about the result
			resultResource = new(map[string]interface{})
		}
		headers := new(fhirclient.Headers)
		if err := fhirClient.Read(*entry.Response.Location, resultResource, fhirclient.ResponseHeaders(headers)); err != nil {
			return errors.Join(ErrEntryNotFound, fmt.Errorf("failed to re-retrieve result Bundle entry (resource=%s): %w", *entry.Response.Location, err))
		}

		if httpResponse == nil {
			return nil // Only execute the transaction and fill the `resultResource`, caller doesn't want to set the response
		}

		for key, value := range headers.Header {
			httpResponse.Header()[key] = value
		}

		if respondWithEntry {
			statusParts := strings.Split(entry.Response.Status, " ")
			status, _ := strconv.Atoi(statusParts[0])
			if status == 0 {
				status = http.StatusOK
			}
			httpResponse.WriteHeader(status)
			return json.NewEncoder(httpResponse).Encode(resultResource)
		}

		httpResponse.WriteHeader(http.StatusOK)
		return json.NewEncoder(httpResponse).Encode(resultBundle)
	}

	return ErrEntryNotFound
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
