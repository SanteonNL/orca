package otel

type EventNames struct{}

const (
	FHIRTransactionPrepare           = "fhir_transaction.prepare"
	FHIRTransactionExecute           = "fhir_transaction.execute"
	FHIRTransactionProcessingResults = "fhir_transaction.processing_results"
	FHIRTransactionComplete          = "fhir_transaction.complete"
)

var Events = EventNames{}
