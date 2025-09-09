package otel

type EventNames struct{}

const (
	CommitTransaction       = "commit_transaction"
	CommitTransactionFailed = "commit_transaction.failed"
	CommitTransactionDone   = "commit_transaction.done"

	ExtractContextInfo                    = "extract_context_info"
	ExtractContextInfoTenantFailed        = "extract_context_info.tenant.failed"
	ExtractContextInfoPrincipalFailed     = "extract_context_info.principal.failed"
	ExtractContextInfoLocalIdentityFailed = "extract_context_info.local_identity.failed"

	FHIRTransactionPrepare           = "fhir_transaction.prepare"
	FHIRTransactionExecute           = "fhir_transaction.execute"
	FHIRTransactionFailed            = "fhir_transaction.failed"
	FHIRTransactionProcessingResults = "fhir_transaction.processing_results"
	FHIRTransactionComplete          = "fhir_transaction.complete"

	RequestReadingBody       = "request.reading_body"
	RequestReadingBodyFailed = "request.reading_body.failed"
)

var Events = EventNames{}
