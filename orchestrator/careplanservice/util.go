package careplanservice

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

// Writes an OperationOutcome based on the given error as HTTP response.
func (s *Service) writeOperationOutcomeFromError(err error, desc string, httpResponse http.ResponseWriter) {
	diagnostics := fmt.Sprintf("%s failed: %s", desc, err.Error())

	issue := fhir.OperationOutcomeIssue{
		Id:                nil,
		Extension:         nil,
		ModifierExtension: nil,
		Severity:          fhir.IssueSeverityError,
		Code:              fhir.IssueTypeProcessing,
		Details:           nil,
		Diagnostics:       to.Ptr(diagnostics),
		Location:          nil,
		Expression:        nil,
	}

	outcome := fhir.OperationOutcome{
		Id:                nil,
		Meta:              nil,
		ImplicitRules:     nil,
		Language:          nil,
		Text:              nil,
		Extension:         nil,
		ModifierExtension: nil,
		Issue:             []fhir.OperationOutcomeIssue{issue},
	}

	httpResponse.Header().Add("Content-Type", "application/fhir+json")
	httpResponse.WriteHeader(http.StatusBadRequest)

	data, err := json.Marshal(outcome)
	if err != nil {
		log.Error().Msgf("Failed to marshal OperationOutcome: %s", diagnostics)
		return
	}

	_, err = httpResponse.Write(data)
	if err != nil {
		log.Error().Msgf("Failed to return OperationOutcome: %s", diagnostics)
		return
	}
}
