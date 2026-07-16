package otel

type EventNames struct{}

const (
	FHIRTransactionPrepare           = "fhir_transaction.prepare"
	FHIRTransactionExecute           = "fhir_transaction.execute"
	FHIRTransactionProcessingResults = "fhir_transaction.processing_results"
	FHIRTransactionComplete          = "fhir_transaction.complete"

	// Generic handler lifecycle (careplanservice/service.go)
	HandlerInvoke         = "handler.invoke"
	HandlerInvokeComplete = "handler.invoke.complete"

	// Task update flow (careplanservice/handle_updatetask.go)
	LookupTaskByQueryParameters = "lookup_task_by_query_parameters"
	LookupTaskByID              = "lookup_task_by_id"
	UpsertTaskCreation          = "upsert_task_creation"
	UpdateExistingTask          = "update_existing_task"
	ValidatingStatusTransition  = "validating_status_transition"
	ResolvingCarePlanReference  = "resolving_careplan_reference"
	AddingTaskUpdateToTx        = "adding_task_update_to_transaction"

	// Task create flow (careplanservice/handle_createtask.go)
	CreatingNewCarePlanAndCareTeam = "creating_new_careplan_and_careteam"
	ActivatingCareTeamMembership   = "activating_careteam_membership"
	AddingCarePlanAndTaskToTx      = "adding_careplan_and_task_to_transaction_bundle"
	NotifyingCarePlanCreatedEvent  = "notifying_careplan_created_event"
	AddingTaskToExistingCarePlan   = "adding_task_to_existing_careplan"
	ValidatingPatientConsistency   = "validating_patient_consistency"
	ProcessingScpSubtask           = "processing_scp_subtask"
	ProcessingPrimaryTask          = "processing_primary_task"
	UpdatingCarePlanActivities     = "updating_careplan_activities"
	AddCarePlanUpdateToTx          = "add_careplan_update_to_transaction"

	// CareTeam service (careplanservice/careteamservice/careteam.go)
	// UpdatingCareTeam is shared with handle_updatetask.go.
	SkippingSubtask               = "skipping_subtask"
	FetchingCarePlanAndActivities = "fetching_careplan_and_activities"
	UpdatingCareTeam              = "updating_careteam"
	ActivatingMembership          = "activating_membership"
	MemberAlreadyInCareTeam       = "member_already_in_careteam"
	AddingMemberToCareTeam        = "adding_member_to_careteam"
	DeactivatingMembership        = "deactivating_membership"
	MemberStillActiveInOtherTasks = "member_still_active_in_other_tasks"
	SettingEndDateForMember       = "setting_end_date_for_member"

	// Zorgplatform SAML validation (careplancontributor/applaunch/zorgplatform/handle_validation.go)
	DecryptedAssertion          = "decrypted assertion"
	ValidatedAssertionSignature = "validated assertion signature"
	ValidatedAssertionAudience  = "validated assertion audience"
	ValidatedAssertionIssuer    = "validated assertion issuer"
	SAMLAssertionParsedSuccess  = "SAML Assertion parsed successfully"

	// Response pipeline (lib/coolfhir/pipeline/pipeline.go)
	ResponseBodyWrite         = "response_body.write"
	ResponseBodyWriteComplete = "response_body.write.complete"
)

var Events = EventNames{}
