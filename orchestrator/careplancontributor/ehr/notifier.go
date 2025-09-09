//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"bytes"
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
)

var _ events.Type = &TaskAcceptedEvent{}

type TaskAcceptedEvent struct {
	FHIRBaseURL string    `json:"fhirBaseURL"`
	Task        fhir.Task `json:"task"`
	TenantID    string    `json:"tenantId"`
}

func (t TaskAcceptedEvent) Entity() messaging.Entity {
	return messaging.Entity{
		Name:   "orca.taskengine.task-accepted",
		Prefix: true,
	}
}

func (t TaskAcceptedEvent) Instance() events.Type {
	return &TaskAcceptedEvent{}
}

// Notifier is an interface for sending notifications regarding task acceptance within a FHIR-based system.
type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error
}

// notifier is a type that uses a ServiceBusClient to send messages to a message broker.
type notifier struct {
	tenants                    tenants.Config
	eventManager               events.Manager
	fhirClientFactory          func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
	taskAcceptedBundleEndpoint string
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(eventManager events.Manager, tenants tenants.Config, taskAcceptedBundleEndpoint string, fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)) (Notifier, error) {
	n := &notifier{
		eventManager:               eventManager,
		fhirClientFactory:          fhirClientFactory,
		tenants:                    tenants,
		taskAcceptedBundleEndpoint: taskAcceptedBundleEndpoint,
	}
	if err := n.start(); err != nil {
		return nil, err
	}
	return n, nil
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a message broker.
func (n *notifier) NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String(otel.FHIRBaseURL, fhirBaseURL),
			attribute.String(otel.FHIRTaskID, *task.Id),
			attribute.String(otel.FHIRTaskStatus, task.Status.Code()),
		),
	)
	defer span.End()

	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return otel.Error(span, err)
	}

	// Process the task notification synchronously to propagate errors
	event := TaskAcceptedEvent{
		TenantID:    tenant.ID,
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	}

	span.SetStatus(codes.Ok, "")
	return n.processTaskAcceptedEvent(ctx, &event)
}

// processTaskAcceptedEvent handles the actual task processing synchronously
func (n *notifier) processTaskAcceptedEvent(ctx context.Context, event *TaskAcceptedEvent) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String(otel.FHIRBaseURL, event.FHIRBaseURL),
			attribute.String(otel.FHIRTaskID, *event.Task.Id),
			attribute.String(otel.FHIRTaskStatus, event.Task.Status.Code()),
		),
	)
	defer span.End()

	// Lookup tenant
	tenant, err := n.tenants.Get(event.TenantID)
	if err != nil {
		return otel.Error(span, errors.Wrapf(err, "failed to get from task accepted event (tenant-id=%s)", event.TenantID))
	}
	ctx = tenants.WithTenant(ctx, *tenant)

	fhirBaseURL, err := url.Parse(event.FHIRBaseURL)
	if err != nil {
		return otel.Error(span, err)
	}

	cpsClient, _, err := n.fhirClientFactory(ctx, fhirBaseURL)
	if err != nil {
		return otel.Error(span, errors.Wrap(err, "failed to create FHIR client for invoking CarePlanService"))
	}

	bundles, err := TaskNotificationBundleSet(ctx, cpsClient, *event.Task.Id)
	if err != nil {
		return otel.Error(span, errors.Wrap(err, "failed to create task notification bundle"))
	}
	log.Ctx(ctx).Info().Msgf("Sending set for task notifier started")

	err = sendBundle(ctx, n.taskAcceptedBundleEndpoint, *bundles)
	if err != nil {
		var badRequest *BadRequest
		if errors.As(err, &badRequest) {
			// Handle BadRequest error specifically
			log.Ctx(ctx).Warn().Err(err).Msg("Task enrollment failed due to bad request")
			task := event.Task
			task.Status = fhir.TaskStatusRejected
			task.StatusReason = &fhir.CodeableConcept{
				Text: badRequest.Reason,
			}
			err = cpsClient.UpdateWithContext(ctx, "Task/"+*event.Task.Id, task, &task)
			if err != nil {
				return otel.Error(span, errors.Wrap(err, "failed to update task status"))
			}
			return nil
		}
		return err
	}

	span.SetStatus(codes.Ok, "")
	return nil
}

func (n *notifier) start() error {
	return n.eventManager.Subscribe(TaskAcceptedEvent{}, func(ctx context.Context, rawEvent events.Type) error {
		event := rawEvent.(*TaskAcceptedEvent)
		return n.processTaskAcceptedEvent(ctx, event)
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, taskAcceptedBundleEndpoint string, set BundleSet) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String(otel.FHIRBundleSetId, set.Id),
			attribute.String("bundle_set.task", set.task),
			attribute.Int(otel.FHIRBundlesCount, len(set.Bundles)),
		),
	)
	defer span.End()

	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return otel.Error(span, err, "failed to serialize bundle set")
	}
	span.SetAttributes(
		attribute.Int("payload.size_bytes", len(jsonData)),
	)

	log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to HTTP endpoint (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "POST", taskAcceptedBundleEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return otel.Error(span, errors.Wrap(err, "failed to create HTTP request"))
	}
	req.Header.Set("Content-Type", "application/fhir+json")

	// Inject trace context into request headers
	baseotel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	httpResponse, err := otel.NewTracedHTTPClient("ehr.notifier").Do(req)
	if err != nil {
		log.Ctx(ctx).Warn().Err(err).Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s) e", set.task, taskAcceptedBundleEndpoint)
		return otel.Error(span, errors.Wrap(err, "failed to send task to endpoint"))
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode == http.StatusBadRequest {
		var badRequest BadRequest
		var operationOutcome fhir.OperationOutcome
		if err := json.NewDecoder(httpResponse.Body).Decode(&operationOutcome); err != nil {
			return otel.Error(span, errors.Wrap(err, "failed to decode bad request response"))
		}
		if len(operationOutcome.Issue) > 0 {
			badRequest.Reason = operationOutcome.Issue[0].Diagnostics
		} else {
			unknownError := "unknown error"
			badRequest.Reason = &unknownError
		}
		return &BadRequest{Reason: badRequest.Reason}
	}
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return otel.Error(span, errors.Errorf("failed to send task to endpoint, status code: %d", httpResponse.StatusCode))
	}
	log.Ctx(ctx).Info().Msgf("Notified EHR of accepted Task with bundle (ref=%s)", set.task)

	span.SetStatus(codes.Ok, "")
	return nil
}

type BadRequest struct {
	Reason *string
}

func (e *BadRequest) Error() string {
	if e.Reason != nil {
		return *e.Reason
	}
	return ""
}
