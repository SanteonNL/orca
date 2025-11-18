package coolfhir

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func TestWriteOperationOutcomeFromError(t *testing.T) {
	type args struct {
		err  error
		desc string
	}
	tests := []struct {
		name         string
		args         args
		expectedCode int
		expectedBody string
	}{
		{
			name: "ErrorWithCode",
			args: args{
				err:  NewErrorWithCode("oops", 400),
				desc: "test",
			},
			expectedCode: 400,
			expectedBody: `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"processing","diagnostics":"test failed: oops"}]}`,
		},
		{
			name: "ErrorWithCode, no code (default 500)",
			args: args{
				err:  NewErrorWithCode("oops", 0),
				desc: "test",
			},
			expectedCode: 500,
			expectedBody: `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"processing","diagnostics":"Internal Server Error"}]}`,
		},
		{
			name: "OperationOutcomeError",
			args: args{
				err: &fhirclient.OperationOutcomeError{
					OperationOutcome: fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{
							{
								Severity:    fhir.IssueSeverityError,
								Code:        fhir.IssueTypeConflict,
								Diagnostics: to.Ptr("oops"),
							},
						},
					},
					HttpStatusCode: 404,
				},
				desc: "test",
			},
			expectedCode: 404,
			expectedBody: `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"conflict","diagnostics":"oops"}]}`,
		},
		{
			name: "OperationOutcomeError, no code (default 500)",
			args: args{
				err: &fhirclient.OperationOutcomeError{
					OperationOutcome: fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{
							{
								Severity:    fhir.IssueSeverityError,
								Code:        fhir.IssueTypeConflict,
								Diagnostics: to.Ptr("oops"),
							},
						},
					},
				},
				desc: "test",
			},
			expectedCode: 500,
			expectedBody: `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"conflict","diagnostics":"oops"}]}`,
		},
		{
			name: "OperationOutcomeError, validation errors 400",
			args: args{
				err: &fhirclient.OperationOutcomeError{
					OperationOutcome: fhir.OperationOutcome{
						Issue: []fhir.OperationOutcomeIssue{{
							Severity:    fhir.IssueSeverityError,
							Code:        fhir.IssueTypeInvalid,
							Diagnostics: to.Ptr("Validation 1 failed"),
						},
							{
								Severity:    fhir.IssueSeverityError,
								Code:        fhir.IssueTypeInvalid,
								Diagnostics: to.Ptr("Validation 2 failed"),
							}},
					},
					HttpStatusCode: http.StatusBadRequest,
				},
				desc: "test",
			},
			expectedCode: 400,
			expectedBody: `{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"invalid","diagnostics":"Validation 1 failed"},{"severity":"error","code":"invalid","diagnostics":"Validation 2 failed"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			WriteOperationOutcomeFromError(context.Background(), tt.args.err, tt.args.desc, response)
			assert.Equal(t, tt.expectedCode, response.Code)
			assert.JSONEq(t, tt.expectedBody, response.Body.String())
		})
	}
}

func TestSanitizeOperationOutcome(t *testing.T) {
	sanitizedCodes := []fhir.IssueType{
		fhir.IssueTypeSecurity,
		fhir.IssueTypeLogin,
		fhir.IssueTypeUnknown,
		fhir.IssueTypeExpired,
		fhir.IssueTypeForbidden,
		fhir.IssueTypeSuppressed,
	}
	nonSanitizedCodes := []fhir.IssueType{}
	for i := 0; i < 30; i++ { // 30 = highest IssueType value
		if !slices.Contains(sanitizedCodes, fhir.IssueType(i)) {
			nonSanitizedCodes = append(nonSanitizedCodes, fhir.IssueType(i))
		}
	}

	for _, code := range sanitizedCodes {
		t.Run(code.String()+" should be sanitized", func(t *testing.T) {
			issue := fhir.OperationOutcomeIssue{
				Code:        code,
				Diagnostics: to.Ptr("secret details"),
			}
			ooc := fhir.OperationOutcome{
				Issue: []fhir.OperationOutcomeIssue{issue},
			}
			sanitized := SanitizeOperationOutcome(ooc)
			assert.Len(t, sanitized.Issue, 1)
			assert.Equal(t, fhir.IssueTypeProcessing, sanitized.Issue[0].Code)
			assert.Equal(t, "upstream FHIR server error", *sanitized.Issue[0].Diagnostics)
		})
	}
	for _, code := range nonSanitizedCodes {
		t.Run(code.String()+" should not be sanitized", func(t *testing.T) {
			issue := fhir.OperationOutcomeIssue{
				Code:        code,
				Diagnostics: to.Ptr("some error message"),
			}
			ooc := fhir.OperationOutcome{
				Issue: []fhir.OperationOutcomeIssue{issue},
			}
			sanitized := SanitizeOperationOutcome(ooc)
			assert.Len(t, sanitized.Issue, 1)
			assert.Equal(t, code, sanitized.Issue[0].Code)
			assert.Equal(t, "some error message", *sanitized.Issue[0].Diagnostics)
		})
	}
}
