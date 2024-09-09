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
	log.Info().Msgf("%s failed: %v", desc, err)
	diagnostics := fmt.Sprintf("%s failed: %s", desc, err.Error())

	issue := fhir.OperationOutcomeIssue{
		Severity:    fhir.IssueSeverityError,
		Code:        fhir.IssueTypeProcessing,
		Diagnostics: to.Ptr(diagnostics),
	}

	outcome := fhir.OperationOutcome{
		Issue: []fhir.OperationOutcomeIssue{issue},
	}

	httpResponse.Header().Add("Content-Type", "application/fhir+json")
	httpResponse.WriteHeader(http.StatusBadRequest)

	data, err := json.Marshal(outcome)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to marshal OperationOutcome: %s", diagnostics)
		return
	}

	_, err = httpResponse.Write(data)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to return OperationOutcome: %s", diagnostics)
	}
}
