package coolfhir

// URANamingSystem is the FHIR NamingSystem URI for the URA
const URANamingSystem = "http://fhir.nl/fhir/NamingSystem/ura"

// BSNNamingSystem is the FHIR NamingSystem URI for the Dutch Social Security Number
const BSNNamingSystem = "http://fhir.nl/fhir/NamingSystem/bsn"

// TypeOrganization is the FHIR type for an Organization
const TypeOrganization = "Organization"

// FHIRContentType is the content-type for FHIR payloads
const FHIRContentType = "application/fhir+json"

// SCPTaskProfile contains the canonical reference of the Shared Care Planning StructureDefinition. Used in the Task.meta.profile field.
const SCPTaskProfile = "http://santeonnl.github.io/shared-care-planning/StructureDefinition/SCPTask"
