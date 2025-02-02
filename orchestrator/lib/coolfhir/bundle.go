package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
		Url:    path,
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
		entry.FullUrl = to.NilString(fullUrl)
	}
}

func WithRequestHeaders(header http.Header) BundleEntryOption {
	return func(entry *fhir.BundleEntry) {
		if entry.Request == nil {
			entry.Request = &fhir.BundleEntryRequest{}
		}
		if header[IfNoneExistHeader] != nil {
			entry.Request.IfNoneExist = to.Ptr(header.Get(IfNoneExistHeader))
		}
		if header[IfMatchHeader] != nil {
			entry.Request.IfMatch = to.Ptr(header.Get(IfMatchHeader))
		}
		if header[IfNoneMatchHeader] != nil {
			entry.Request.IfNoneMatch = to.Ptr(header.Get(IfNoneMatchHeader))
		}
		if header[IfModifiedSinceHeader] != nil {
			entry.Request.IfModifiedSince = to.Ptr(header.Get(IfModifiedSinceHeader))
		}
	}
}

func HeadersFromBundleEntryRequest(entry *fhir.BundleEntryRequest) http.Header {
	header := http.Header{}
	if entry.IfNoneExist != nil {
		header.Set(IfNoneExistHeader, *entry.IfNoneExist)
	}
	if entry.IfMatch != nil {
		header.Set(IfMatchHeader, *entry.IfMatch)
	}
	if entry.IfNoneMatch != nil {
		header.Set(IfNoneMatchHeader, *entry.IfNoneMatch)
	}
	if entry.IfModifiedSince != nil {
		header.Set(IfModifiedSinceHeader, *entry.IfModifiedSince)
	}
	return header
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

// NormalizeTransactionBundleResponseEntry normalizes a transaction bundle response entry returned from an upstream FHIR server,
// so it can be returned to a client, who is agnostic of the upstream FHIR server implementation.
// It does the following:
// - Change the response.location property to a relative URL if it was an absolute URL
// - Read the resource being referenced and unmarshal it into the given result argument (so it can be used for notification).
// - Set the response.resource property to the read resource
func NormalizeTransactionBundleResponseEntry(ctx context.Context, fhirClient fhirclient.Client, fhirBaseURL *url.URL, requestEntry *fhir.BundleEntry, responseEntry *fhir.BundleEntry, result interface{}) (*fhir.BundleEntry, error) {
	if responseEntry.Response == nil {
		return nil, errors.New("entry.Response is nil")
	}
	resultEntry := *responseEntry
	// Enrich result with resource from FHIR server
	if resultEntry.Resource == nil {
		// Microsoft Azure FHIR:
		// - When PUT-ing a resource, the resultEntry entry might not contain a location.
		//   In that case, the location is the same as the request URL.
		// - When POST-ing a resource with IfNoneExist and the resource already exists, the resource itself isn't returned.
		//   It only returns: {"response":{"status":"200"}}
		//   In theory, this could be fixed with the 'Prefer: return=representation' header, but Azure FHIR doesn't support it.
		//   See https://github.com/microsoft/fhir-server/issues/2431
		var resourcePath string
		var searchParams url.Values
		if resultEntry.Response.Location != nil {
			resourcePath = *resultEntry.Response.Location
			// HAPI uses relative Location URLs, Microsoft Azure FHIR uses absolute URLs.
			resourcePath = strings.TrimPrefix(resourcePath, fhirBaseURL.String())
			// depending on the base URL ending with slash or not, we might end up with a leading slash.
			// Trim it for deterministic comparison.
			resourcePath = strings.TrimPrefix(resourcePath, "/")
			// Consistent behavior for easier testing and integration: pass the relative resource URL to the FHIR client.
			// (HAPI uses relative Location URLs, Microsoft Azure FHIR uses absolute URLs.)
			resultEntry.Response.Location = to.Ptr(resourcePath)
		} else if strings.Contains(requestEntry.Request.Url, "/") {
			// resultEntry.location is not set, might be an upsert with logical identifier.
			// In this case, it's a literal reference
			resourcePath = requestEntry.Request.Url
		} else if strings.Contains(requestEntry.Request.Url, "?") {
			// resultEntry.location is not set, might be an upsert with logical identifier.
			// In this case, it's a reference with a logical identifier
			entryRequestUrl, err := url.Parse(requestEntry.Request.Url)
			if err != nil {
				return nil, err
			}
			resourcePath = entryRequestUrl.Path
			searchParams = entryRequestUrl.Query()
		} else if requestEntry.Request.IfNoneExist != nil {
			// Azure FHIR behavior: conditional create matched existing resource, Azure FHIR only returns "200 OK".
			// Need to find the existing resource by the IfNoneExist query parameters.
			q, err := url.Parse("?" + *requestEntry.Request.IfNoneExist)
			if err != nil {
				return nil, err
			}
			resourcePath = requestEntry.Request.Url
			searchParams = q.Query()
		}

		if resourcePath == "" {
			responseBundleEntryJson, _ := json.Marshal(responseEntry)
			log.Error().Msgf("Failed to determine resource path from FHIR transaction resultEntry bundle: %s", string(responseBundleEntryJson))
			return nil, errors.New("failed to determine resource for transaction response bundle entry, see log for more details")
		}
		if len(searchParams) == 0 {
			var resourceData []byte
			if err := fhirClient.ReadWithContext(ctx, resourcePath, &resourceData); err != nil {
				return nil, errors.Join(ErrEntryNotFound, fmt.Errorf("failed to retrieve result Bundle entry (resource=%s): %w", resourcePath, err))
			}
			resultEntry.Resource = resourceData
		} else {
			var searchResultBundle fhir.Bundle
			if err := fhirClient.SearchWithContext(ctx, resourcePath, searchParams, &searchResultBundle); err != nil {
				return nil, errors.Join(ErrEntryNotFound, fmt.Errorf("failed to search for result Bundle entry (resource=%s): %w", resourcePath, err))
			}
			if len(searchResultBundle.Entry) == 0 {
				return nil, errors.Join(ErrEntryNotFound, fmt.Errorf("no result Bundle entry found (resource=%s)", resourcePath))
			}
			if len(searchResultBundle.Entry) > 1 {
				return nil, errors.New("multiple result Bundle entries found, expected 1")
			}
			resultEntry.Resource = searchResultBundle.Entry[0].Resource
		}
	}
	if len(resultEntry.Resource) != 0 && result != nil {
		if err := json.Unmarshal(resultEntry.Resource, result); err != nil {
			return nil, fmt.Errorf("unmarshal Bundle entry (target=%T): %w", result, err)
		}
	}
	return &resultEntry, nil
}
