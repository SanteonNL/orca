package careplancontributor

import (
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"strconv"
	"strings"
)

func (s *Service) handleBatch(httpRequest *http.Request, requestBundle fhir.Bundle) (*fhir.Bundle, error) {
	if s.ehrFhirProxy == nil {
		return nil, coolfhir.BadRequest("EHR API is not supported")
	}
	log.Ctx(httpRequest.Context()).Debug().Msg("Handling external FHIR API request")
	_, err := s.authorizeScpMember(httpRequest)
	if err != nil {
		return nil, err
	}
	return s.doHandleBatch(httpRequest, requestBundle)
}

func (s *Service) doHandleBatch(httpRequest *http.Request, requestBundle fhir.Bundle) (*fhir.Bundle, error) {
	responseBundle := coolfhir.BatchResponse()
	for _, requestEntry := range requestBundle.Entry {
		if requestEntry.Request == nil || requestEntry.Request.Method != fhir.HTTPVerbGET {
			responseBundle.AppendOperationOutcome(http.StatusBadRequest, fhir.OperationOutcomeIssue{
				Severity: fhir.IssueSeverityError,
				Code:     fhir.IssueTypeNotSupported,
				Details: &fhir.CodeableConcept{
					Text: to.Ptr("Only GET requests are supported in batch processing"),
				},
			})
			continue
		}
		requestURL := must.ParseURL(requestEntry.Request.Url)
		var responseStatusCode int
		var responseData []byte
		requestOpts := []fhirclient.Option{
			fhirclient.ResponseStatusCode(&responseStatusCode),
			fhirclient.RequestHeaders(map[string][]string{
				// We need to propagate the X-Scp-Context header to FHIR client doing the request,
				// Zorgplatform STS RoundTripper needs it.
				"X-Scp-Context": {httpRequest.Header.Get("X-Scp-Context")},
			}),
		}
		var err error
		if !strings.Contains(requestEntry.Request.Url, "/") {
			// It's a search operation
			err = s.ehrFhirClient.SearchWithContext(httpRequest.Context(), requestURL.Path, requestURL.Query(), &responseData, requestOpts...)
		} else {
			// It's a read operation
			err = s.ehrFhirClient.ReadWithContext(httpRequest.Context(), requestURL.Path, &responseData, requestOpts...)
		}
		if err != nil {
			var opOutcomeErr fhirclient.OperationOutcomeError
			if errors.As(err, &opOutcomeErr) {
				// A FHIR error occurred, return it as operation outcome
				responseBundle.AppendEntry(fhir.BundleEntry{
					Response: &fhir.BundleEntryResponse{
						Status:  strconv.Itoa(opOutcomeErr.HttpStatusCode) + " " + http.StatusText(opOutcomeErr.HttpStatusCode),
						Outcome: must.MarshalJSON(opOutcomeErr.OperationOutcome),
					},
				})
			} else {
				// Another error occurred
				responseBundle.AppendOperationOutcome(http.StatusBadGateway, fhir.OperationOutcomeIssue{
					Severity: fhir.IssueSeverityWarning,
					Code:     fhir.IssueTypeProcessing,
					Details: &fhir.CodeableConcept{
						Text: to.Ptr("Upstream FHIR server error: " + err.Error()),
					},
				})
			}
		} else {
			responseBundle.AppendEntry(fhir.BundleEntry{
				Response: &fhir.BundleEntryResponse{
					Status: strconv.Itoa(responseStatusCode) + " " + http.StatusText(responseStatusCode),
				},
				Resource: responseData,
			})
		}
	}
	return to.Ptr(responseBundle.Bundle()), nil
}
