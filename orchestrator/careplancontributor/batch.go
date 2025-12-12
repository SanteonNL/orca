package careplancontributor

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func (s *Service) handleFHIRBatchBundle(httpRequest *http.Request, requestBundle fhir.Bundle) (*fhir.Bundle, error) {
	tenant, err := tenants.FromContext(httpRequest.Context())
	if err != nil {
		return nil, err
	}
	fhirClient := s.ehrFHIRClientByTenant[tenant.ID]
	if fhirClient == nil {
		return nil, coolfhir.BadRequest("EHR API is not supported")
	}
	ctx, span := tracer.Start(
		httpRequest.Context(),
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String(otel.HTTPMethod, httpRequest.Method),
			attribute.String(otel.HTTPURL, httpRequest.URL.String()),
			attribute.String(otel.FHIRBundleType, requestBundle.Type.String()),
			attribute.Int(otel.FHIRBundleEntryCount, len(requestBundle.Entry)),
		),
	)
	defer span.End()

	slog.DebugContext(ctx, "Handling external FHIR API request")

	_, err = s.authorizeScpMember(httpRequest.WithContext(ctx))
	if err != nil {
		return nil, otel.Error(span, err)
	}

	result, err := s.doHandleBatch(httpRequest.WithContext(ctx), requestBundle, fhirClient)
	if err != nil {
		return nil, otel.Error(span, err)
	}

	span.SetStatus(codes.Ok, "")
	return result, nil
}

func (s *Service) doHandleBatch(httpRequest *http.Request, requestBundle fhir.Bundle, fhirClient fhirclient.Client) (*fhir.Bundle, error) {
	responseBundle := coolfhir.BatchResponse()
	// This looks complicated, but is to support parallel execution of the bundle entries;
	// entries in the response bundle need to be in the same order as the request entries.
	type entryResult struct {
		index                      int
		entry                      *fhir.BundleEntry
		operationOutcomeIssue      *fhir.OperationOutcomeIssue
		operationOutcomeStatusCode *int
	}
	outcomesChan := make(chan entryResult, len(requestBundle.Entry))
	for idx, requestEntry := range requestBundle.Entry {
		fn := func(index int, requestEntry fhir.BundleEntry) entryResult {
			if requestEntry.Request == nil || requestEntry.Request.Method != fhir.HTTPVerbGET {
				return entryResult{
					index: index,
					operationOutcomeIssue: &fhir.OperationOutcomeIssue{
						Severity: fhir.IssueSeverityError,
						Code:     fhir.IssueTypeNotSupported,
						Details: &fhir.CodeableConcept{
							Text: to.Ptr("Only GET requests are supported in batch processing"),
						},
					},
					operationOutcomeStatusCode: to.Ptr(http.StatusBadRequest),
				}
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
				err = fhirClient.SearchWithContext(httpRequest.Context(), requestURL.Path, requestURL.Query(), &responseData, requestOpts...)
			} else {
				// It's a read operation
				err = fhirClient.ReadWithContext(httpRequest.Context(), requestURL.Path, &responseData, requestOpts...)
			}
			if err != nil {
				var opOutcomeErr fhirclient.OperationOutcomeError
				if errors.As(err, &opOutcomeErr) {
					// A FHIR error occurred, return it as operation outcome
					return entryResult{
						index: index,
						entry: &fhir.BundleEntry{
							Response: &fhir.BundleEntryResponse{
								Status:  strconv.Itoa(opOutcomeErr.HttpStatusCode) + " " + http.StatusText(opOutcomeErr.HttpStatusCode),
								Outcome: must.MarshalJSON(opOutcomeErr.OperationOutcome),
							},
						},
					}
				} else {
					// Another error occurred
					return entryResult{
						index:                      index,
						operationOutcomeStatusCode: to.Ptr(http.StatusBadGateway),
						operationOutcomeIssue: &fhir.OperationOutcomeIssue{
							Severity: fhir.IssueSeverityWarning,
							Code:     fhir.IssueTypeProcessing,
							Details: &fhir.CodeableConcept{
								Text: to.Ptr("Upstream FHIR server error: " + err.Error()),
							},
						},
					}
				}
			} else {
				return entryResult{
					index: index,
					entry: &fhir.BundleEntry{
						Response: &fhir.BundleEntryResponse{
							Status: strconv.Itoa(responseStatusCode) + " " + http.StatusText(responseStatusCode),
						},
						Resource: responseData,
					},
				}
			}
		}

		if s.config.ParallelBatch {
			go func(requestEntry fhir.BundleEntry) {
				outcomesChan <- fn(idx, requestEntry)
			}(requestEntry)
		} else {
			outcomesChan <- fn(idx, requestEntry)
		}
	}

	// Collect outcomes, sort
	outcomes := make([]entryResult, len(requestBundle.Entry))
	for i := 0; i < len(requestBundle.Entry); i++ {
		result := <-outcomesChan
		outcomes[result.index] = result
	}
	close(outcomesChan)
	// Build response bundle
	for _, outcome := range outcomes {
		if outcome.operationOutcomeIssue != nil {
			responseBundle.AppendOperationOutcome(*outcome.operationOutcomeStatusCode, *outcome.operationOutcomeIssue)
		} else {
			responseBundle.AppendEntry(*outcome.entry)
		}
	}

	return to.Ptr(responseBundle.Bundle()), nil
}
