package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// SanitizeOperationOutcome removes security-related information from an OperationOutcome, replacing it with a generic message,
// so that it can be safely returned to the client.
// It follows the code list from the FHIR specification: https://www.hl7.org/fhir/codesystem-issue-type.html#issue-type-security
func SanitizeOperationOutcome(in fhir.OperationOutcome) fhir.OperationOutcome {
	result := in
	result.Issue = nil
	for _, issue := range in.Issue {
		switch issue.Code {
		case fhir.IssueTypeSecurity:
			fallthrough
		case fhir.IssueTypeLogin:
			fallthrough
		case fhir.IssueTypeUnknown:
			fallthrough
		case fhir.IssueTypeExpired:
			fallthrough
		case fhir.IssueTypeForbidden:
			fallthrough
		case fhir.IssueTypeSuppressed:
			result.Issue = append(result.Issue, fhir.OperationOutcomeIssue{
				Severity:    issue.Severity,
				Code:        fhir.IssueTypeProcessing,
				Diagnostics: to.Ptr("upstream FHIR server error"),
			})
		default:
			result.Issue = append(result.Issue, issue)
		}
	}
	return result
}

// ErrorWithCode is a wrapped error struct that can take an error message as well as an HTTP status code
type ErrorWithCode struct {
	Message    string
	StatusCode int
}

func (e ErrorWithCode) Error() string {
	return e.Message
}

// NewErrorWithCode constructs a new ErrorWithCode custom wrapped error
func NewErrorWithCode(message string, statusCode int) error {
	return &ErrorWithCode{
		Message:    message,
		StatusCode: statusCode,
	}
}

// BadRequestError wraps an error with a status code of 400
func BadRequestError(err error) error {
	return &ErrorWithCode{
		Message:    err.Error(),
		StatusCode: http.StatusBadRequest,
	}
}

// BadRequest creates an error with a status code of 400
func BadRequest(msg string, args ...any) error {
	return BadRequestError(fmt.Errorf(msg, args...))
}

// CreateOperationOutcomeBundleEntryFromError creates a BundleEntry with an OperationOutcome based on the given error
func CreateOperationOutcomeBundleEntryFromError(err error, desc string) fhir.BundleEntry {
	rawOperationOutcome, _ := json.Marshal(fhir.OperationOutcome{
		Issue: []fhir.OperationOutcomeIssue{
			{
				Severity:    fhir.IssueSeverityError,
				Diagnostics: to.Ptr(fmt.Sprintf("%s: %v", desc, err)),
			},
		},
	})
	return fhir.BundleEntry{
		Resource: rawOperationOutcome,
	}
}

// WriteOperationOutcomeFromError writes an OperationOutcome based on the given error as HTTP response.
// when sent a WriteOperationOutcomeFromError, it will write the contained error code to the header, else it defaults to StatusBadRequest
func WriteOperationOutcomeFromError(ctx context.Context, err error, desc string, httpResponse http.ResponseWriter) {
	slog.ErrorContext(ctx, fmt.Sprintf("%s failed: %v", desc, err))

	statusCode := http.StatusInternalServerError
	var operationOutcome fhir.OperationOutcome

	// Error type: fhirclient.OperationOutcomeError
	var operationOutcomeErr = new(fhirclient.OperationOutcomeError)
	if errors.As(err, operationOutcomeErr) || errors.As(err, &operationOutcomeErr) {
		if operationOutcomeErr.HttpStatusCode > 0 {
			statusCode = operationOutcomeErr.HttpStatusCode
		}
		operationOutcome = operationOutcomeErr.OperationOutcome
		if statusCode != http.StatusBadRequest {
			operationOutcome = SanitizeOperationOutcome(operationOutcome)
		}
	} else {
		// Error type: ErrorWithCode
		var errorWithCode = new(ErrorWithCode)
		if errors.As(err, errorWithCode) || errors.As(err, &errorWithCode) {
			if errorWithCode.StatusCode > 0 {
				statusCode = errorWithCode.StatusCode
			}
		}

		diagnostics := http.StatusText(statusCode)
		// Include error message for bad requests
		if statusCode == http.StatusBadRequest {
			diagnostics = err.Error()
		}
		operationOutcome = fhir.OperationOutcome{
			Issue: []fhir.OperationOutcomeIssue{
				{
					Severity:    fhir.IssueSeverityError,
					Code:        fhir.IssueTypeProcessing,
					Diagnostics: to.Ptr(fmt.Sprintf("%s failed: %s", desc, diagnostics)),
				},
			},
		}
	}
	SendResponse(httpResponse, statusCode, operationOutcome)
}
