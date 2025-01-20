package coolfhir

import "net/http"

// URANamingSystem is the FHIR NamingSystem URI for the URA
const URANamingSystem = "http://fhir.nl/fhir/NamingSystem/ura"

// BSNNamingSystem is the FHIR NamingSystem URI for the Dutch Social Security Number
const BSNNamingSystem = "http://fhir.nl/fhir/NamingSystem/bsn"

// FHIRContentType is the content-type for FHIR payloads
const FHIRContentType = "application/fhir+json"

// SCPTaskProfile contains the canonical reference of the Shared Care Planning StructureDefinition. Used in the Task.meta.profile field.
const SCPTaskProfile = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"

const IfNoneExistHeader = "If-None-Exist"

const IfMatchHeader = "If-Match"

const IfNoneMatchHeader = "If-None-Match"

const IfModifiedSinceHeader = "If-Modified-Since"

const PreferHeader = "Prefer"

const AcceptHeader = "Accept"

// FilterRequestHeaders filters the given HTTP request headers, returning only those that are specified by https://www.hl7.org/fhir/http.html#Http-Headers
func FilterRequestHeaders(headers http.Header) http.Header {
	filteredHeaders := http.Header{}
	for headerName, headerValues := range headers {
		switch headerName {
		case IfNoneExistHeader, IfMatchHeader, IfNoneMatchHeader, IfModifiedSinceHeader, PreferHeader, AcceptHeader:
			filteredHeaders[headerName] = append(filteredHeaders[headerName], headerValues...)
		}
	}
	return filteredHeaders
}
