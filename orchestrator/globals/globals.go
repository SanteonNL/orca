package globals

import fhirclient "github.com/SanteonNL/go-fhir-client"

var CarePlanServiceFhirClient fhirclient.Client

// StrictMode is a global variable that can be set to true to enable strict mode. If strict mode is enabled,
// potentially unsafe behavior is disabled.
var StrictMode bool
