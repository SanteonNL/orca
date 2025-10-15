package otel

// TraceAttributes contains standardized attribute keys for OpenTelemetry tracing
type TraceAttributes struct{}

// Common attribute keys used across services
const (
	OperationName = "operation.name"

	// HTTP attributes
	HTTPMethod     = "http.method"
	HTTPURL        = "http.url"
	HTTPStatusCode = "http.status_code"
	HTTPStatusText = "http.status_text"

	// FHIR attributes
	FHIRResourceType      = "fhir.resource_type"
	FHIRResourceID        = "fhir.resource_id"
	FHIRResourceReference = "fhir.resource_reference"
	FHIRBaseURL           = "fhir.base_url"
	FHIRSearchParamCount  = "fhir.search.param_count"
	FHIRTaskID            = "fhir.task.id"
	FHIRBundlesCount      = "fhir.bundles.count"
	FHIRBundleSetId       = "fhir.bundle.set_id"
	FHIRTasksCount        = "fhir.tasks.count"
	FHIRTaskStatus        = "fhir.task.status"
	FHIRTaskType          = "fhir.task.type"
	FHIRClientType        = "fhir.client.type"

	// Auth
	AuthZAllowed = "authorization.allowed"
	AuthZReasons = "authorization.reasons"

	// Validation
	ValidationResult = "validation.result"

	// FHIR bundle attributes
	FHIRBundleType               = "fhir.bundle.type"
	FHIRBundleEntryCount         = "fhir.bundle.entry_count"
	FHIRBundleResultEntries      = "fhir.bundle.result_entries"
	FHIRTransactionResultEntries = "fhir.transaction.result_entries"

	// Notification attributes
	NotificationResourceType = "notification.resource_type"
	NotificationShouldNotify = "notification.should_notify"
	NotificationStatus       = "notification.status"
	NotificationResources    = "fhir.notification.resources"

	// Tenant attributes
	TenantID = "tenant.id"
)

var Attributes = TraceAttributes{}
