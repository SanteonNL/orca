package careplancontributor

import (
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (s *Service) handleBatch(httpRequest *http.Request, requestBundle fhir.Bundle) (*fhir.Bundle, error) {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		httpRequest.Context(),
		"handleBatch",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.method", httpRequest.Method),
			attribute.String("http.url", httpRequest.URL.String()),
			attribute.String("operation.name", "CarePlanContributor/HandleBatch"),
			attribute.String("fhir.bundle.type", requestBundle.Type.String()),
			attribute.Int("fhir.bundle.entry_count", len(requestBundle.Entry)),
		),
	)
	defer span.End()

	start := time.Now()

	if s.ehrFhirProxy == nil {
		err := coolfhir.BadRequest("EHR API is not supported")
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	log.Ctx(ctx).Debug().Msg("Handling external FHIR API request")

	_, err := s.authorizeScpMember(httpRequest.WithContext(ctx))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	result, err := s.doHandleBatch(httpRequest.WithContext(ctx), requestBundle)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	span.SetAttributes(
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")
	return result, nil
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
