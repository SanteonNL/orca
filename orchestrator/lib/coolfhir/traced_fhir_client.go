package coolfhir

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/url"
)

type TracedFHIRClient struct {
	client fhirclient.Client
	tracer trace.Tracer
}

func NewTracedFHIRClient(client fhirclient.Client, tracer trace.Tracer) *TracedFHIRClient {
	return &TracedFHIRClient{
		client: client,
		tracer: tracer,
	}
}

func (t *TracedFHIRClient) CreateWithContext(ctx context.Context, resource interface{}, result interface{}, options ...fhirclient.Option) error {
	ctx, span := t.tracer.Start(ctx, "fhir.create",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.operation", "create"),
			attribute.String("fhir.resource_type", ResourceType(resource)),
		),
	)
	defer span.End()

	err := t.client.CreateWithContext(ctx, resource, result, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (t *TracedFHIRClient) Create(resource interface{}, result interface{}, options ...fhirclient.Option) error {
	return t.CreateWithContext(context.Background(), resource, result, options...)
}

func (t *TracedFHIRClient) ReadWithContext(ctx context.Context, path string, result interface{}, options ...fhirclient.Option) error {
	ctx, span := t.tracer.Start(ctx, "fhir.read",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.operation", "read"),
			attribute.String("fhir.path", path),
		),
	)
	defer span.End()

	err := t.client.ReadWithContext(ctx, path, result, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (t *TracedFHIRClient) Read(path string, result interface{}, options ...fhirclient.Option) error {
	return t.ReadWithContext(context.Background(), path, result, options...)
}

func (t *TracedFHIRClient) UpdateWithContext(ctx context.Context, path string, resource interface{}, result interface{}, options ...fhirclient.Option) error {
	ctx, span := t.tracer.Start(ctx, "fhir.update",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.operation", "update"),
			attribute.String("fhir.resource_type", ResourceType(resource)),
		),
	)
	defer span.End()

	err := t.client.UpdateWithContext(ctx, path, resource, result, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (t *TracedFHIRClient) Update(path string, resource interface{}, result interface{}, options ...fhirclient.Option) error {
	return t.UpdateWithContext(context.Background(), path, resource, result, options...)
}

func (t *TracedFHIRClient) DeleteWithContext(ctx context.Context, path string, options ...fhirclient.Option) error {
	ctx, span := t.tracer.Start(ctx, "fhir.delete",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.operation", "delete"),
			attribute.String("fhir.path", path),
		),
	)
	defer span.End()

	err := t.client.DeleteWithContext(ctx, path, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (t *TracedFHIRClient) Delete(path string, options ...fhirclient.Option) error {
	return t.DeleteWithContext(context.Background(), path, options...)
}

func (t *TracedFHIRClient) SearchWithContext(ctx context.Context, resourceType string, params url.Values, result interface{}, options ...fhirclient.Option) error {
	ctx, span := t.tracer.Start(ctx, "fhir.search",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("fhir.operation", "search"),
			attribute.String("fhir.resource_type", resourceType),
			attribute.Int("fhir.search.param_count", len(params)),
		),
	)
	defer span.End()

	err := t.client.SearchWithContext(ctx, resourceType, params, result, options...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
	return err
}

func (t *TracedFHIRClient) Search(resourceType string, params url.Values, result interface{}, options ...fhirclient.Option) error {
	return t.SearchWithContext(context.Background(), resourceType, params, result, options...)
}

func (t *TracedFHIRClient) Path(path ...string) *url.URL {
	return t.client.Path(path...)
}
