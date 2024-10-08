package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

// ErrorWithCode is a wrapped error struct that can take an error message as well as an HTTP status code
type ErrorWithCode struct {
	Message    string
	StatusCode int
}

func (e *ErrorWithCode) Error() string {
	return e.Message
}

// NewErrorWithCode constructs a new ErrorWithCode custom wrapped error
func NewErrorWithCode(message string, statusCode int) error {
	return &ErrorWithCode{
		Message:    message,
		StatusCode: statusCode,
	}
}

// WrapErrorWithCode wraps an error with a status code
func WrapErrorWithCode(err error, statusCode int) error {
	return &ErrorWithCode{
		Message:    err.Error(),
		StatusCode: statusCode,
	}
}

// BadRequestError wraps an error with a status code of 400
func BadRequestError(msg string) error {
	return &ErrorWithCode{
		Message:    msg,
		StatusCode: http.StatusBadRequest,
	}
}

func BadRequestErrorF(msg string, args ...interface{}) error {
	return &ErrorWithCode{
		Message:    fmt.Errorf(msg, args...).Error(),
		StatusCode: http.StatusBadRequest,
	}
}

// WriteOperationOutcomeFromError writes an OperationOutcome based on the given error as HTTP response.
// when sent a WriteOperationOutcomeFromError, it will write the contained error code to the header, else it defaults to StatusBadRequest
func WriteOperationOutcomeFromError(err error, desc string, httpResponse http.ResponseWriter) {
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

	httpResponse.Header().Add("Content-Type", FHIRContentType)
	var errorWithCode *ErrorWithCode
	if errors.As(err, &errorWithCode) {
		httpResponse.WriteHeader(errorWithCode.StatusCode)
	} else {
		httpResponse.WriteHeader(http.StatusBadRequest)
	}

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
