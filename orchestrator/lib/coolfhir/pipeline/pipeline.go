package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"io"
	"net/http"
	"strconv"

	"go.opentelemetry.io/otel/trace"
)

func New() Instance {
	return Instance{}
}

type Instance struct {
	httpResponseTransformers []HttpResponseTransformer
}

// Do executes the pipeline, returning an error if marshalling fails.
func (p Instance) Do(ctx context.Context, tracer trace.Tracer, httpResponse *http.Response, resource any) error {
	ctx, span := tracer.Start(ctx, debug.GetFullCallerName(), trace.WithSpanKind(trace.SpanKindServer))
	defer span.End()

	var responseBody []byte
	if resource != nil {
		var err error
		responseBody, err = marshalResponse(resource)
		if err != nil {
			return otel.Error(span, fmt.Errorf("failed to marshal response: %w", err))
		}
	}

	responseHeaders := httpResponse.Header
	for _, transformer := range p.httpResponseTransformers {
		transformer.Transform(&httpResponse.StatusCode, &responseBody, responseHeaders)
	}

	// Set headers
	if httpResponse.Header.Get("Content-Type") == "" {
		httpResponse.Header.Set("Content-Type", "application/fhir+json")
	}
	if resource != nil {
		httpResponse.Header.Set("Content-Length", strconv.Itoa(len(responseBody)))
		httpResponse.Body = io.NopCloser(bytes.NewReader(responseBody))
	}
	return nil
}

// DoAndWrite executes the pipeline, writing the response to the given HTTP response writer.
// If an error occurs, an internal server error is written to the response.
func (p Instance) DoAndWrite(ctx context.Context, tracer trace.Tracer, httpResponseWriter http.ResponseWriter, resource any, responseStatusCode int) {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("http.status_code", strconv.Itoa(responseStatusCode)),
		),
	)
	defer span.End()

	httpResponse := &http.Response{
		Header:     http.Header{},
		StatusCode: responseStatusCode,
	}

	err := p.Do(ctx, tracer, httpResponse, resource)
	var responseBody []byte
	if err == nil && httpResponse.Body != nil {
		responseBody, err = io.ReadAll(httpResponse.Body)
	}
	if err != nil {
		log.Error().Err(otel.Error(span, err)).Msg("Failed to marshal pipeline response")
		httpResponse.StatusCode = http.StatusInternalServerError
		responseBody = []byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"processing","diagnostics":"Failed to marshal response"}]}`)
	}

	for key, value := range httpResponse.Header {
		httpResponseWriter.Header()[key] = value
	}
	span.SetAttributes(attribute.Int("http.status_code", httpResponse.StatusCode))

	span.AddEvent("response_body.write")
	httpResponseWriter.WriteHeader(httpResponse.StatusCode)
	if responseBody != nil {
		_, err = httpResponseWriter.Write(responseBody)
		if err != nil {
			log.Error().Err(otel.Error(span, err)).Msgf("Failed to write response: %s", string(responseBody))
		}
		span.AddEvent("response_body.write.complete")
	}
}

// AppendResponseTransformer makes a copy of the pipeline and adds the given HTTP response transformer.
// It returns the copy and leaves the original pipeline unchanged.
func (p Instance) AppendResponseTransformer(transformer HttpResponseTransformer) Instance {
	p.httpResponseTransformers = append(p.httpResponseTransformers, transformer)
	return p
}

// PrependResponseTransformer is like AppendResponseTransformer but prepends the transformer to the list of transformers.
func (p Instance) PrependResponseTransformer(transformer HttpResponseTransformer) Instance {
	p.httpResponseTransformers = append([]HttpResponseTransformer{transformer}, p.httpResponseTransformers...)
	return p
}

type HttpResponseTransformer interface {
	Transform(responseStatus *int, responseBody *[]byte, responseHeaders map[string][]string)
}

func marshalResponse(resource any) ([]byte, error) {
	if d, ok := resource.([]byte); ok {
		return d, nil
	} else if reader, ok := resource.(io.Reader); ok {
		return io.ReadAll(reader)
	}
	return json.MarshalIndent(resource, "", "  ")
}
