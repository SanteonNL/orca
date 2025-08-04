//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"bytes"
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
)

var _ events.Type = &TaskAcceptedEvent{}

type TaskAcceptedEvent struct {
	FHIRBaseURL string    `json:"fhirBaseURL"`
	Task        fhir.Task `json:"task"`
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
	eventManager      events.Manager
	broker            messaging.Broker
	fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(eventManager events.Manager, messageBroker messaging.Broker, receiverTopicOrQueue messaging.Entity, taskAcceptedBundleEndpoint string, fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)) (Notifier, error) {
	n := &notifier{
		eventManager:      eventManager,
		broker:            messageBroker,
		fhirClientFactory: fhirClientFactory,
	}
	if err := n.start(receiverTopicOrQueue, taskAcceptedBundleEndpoint); err != nil {
		return nil, err
	}
	return n, nil
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a message broker.
func (n *notifier) NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"NotifyTaskAccepted",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("operation.name", "EHR/NotifyTaskAccepted"),
			attribute.String("fhir.base_url", fhirBaseURL),
			attribute.String("fhir.task_id", *task.Id),
			attribute.String("fhir.task_status", task.Status.Code()),
		),
	)
	defer span.End()

	err := n.eventManager.Notify(ctx, TaskAcceptedEvent{
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to notify task accepted event")
		return err
	}

	return nil
}

func (n *notifier) start(receiverTopicOrQueue messaging.Entity, taskAcceptedBundleEndpoint string) error {
	// TODO: add sync call here to the API of DataHub for enrolling a patient
	// IF the call fails here we want to create a subtask to let the hospital know that the enrollment failed
	return n.eventManager.Subscribe(TaskAcceptedEvent{}, func(ctx context.Context, rawEvent events.Type) error {
		tracer := otel.Tracer(tracerName)
		ctx, span := tracer.Start(
			ctx,
			"handleTaskAcceptedEvent",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("operation.name", "EHR/handleTaskAcceptedEvent"),
				attribute.String("event.type", "TaskAcceptedEvent"),
			),
		)
		defer span.End()

		event := rawEvent.(*TaskAcceptedEvent)
		span.SetAttributes(
			attribute.String("fhir.base_url", event.FHIRBaseURL),
			attribute.String("fhir.task_id", *event.Task.Id),
			attribute.String("fhir.task_status", event.Task.Status.Code()),
		)

		fhirBaseURL, err := url.Parse(event.FHIRBaseURL)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to parse FHIR base URL")
			return err
		}

		cpsClient, _, err := n.fhirClientFactory(ctx, fhirBaseURL)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create FHIR client")
			return errors.Wrap(err, "failed to create FHIR client for invoking CarePlanService")
		}

		bundles, err := TaskNotificationBundleSet(ctx, cpsClient, *event.Task.Id)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to create task notification bundle")
			return errors.Wrap(err, "failed to create task notification bundle")
		}
		log.Ctx(ctx).Info().Msgf("Sending set for task notifier started")

		err = sendBundle(ctx, receiverTopicOrQueue, taskAcceptedBundleEndpoint, *bundles, n.broker)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to send bundle")
			return err
		}

		span.SetAttributes(
			attribute.String("bundle_set.id", bundles.Id),
			attribute.Int("bundles.count", len(bundles.Bundles)),
		)

		return nil
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, receiverTopicOrQueue messaging.Entity, taskAcceptedBundleEndpoint string, set BundleSet, messageBroker messaging.Broker) error {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"sendBundle",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("operation.name", "EHR/sendBundle"),
			attribute.String("bundle_set.id", set.Id),
			attribute.String("bundle_set.task", set.task),
			attribute.Int("bundles.count", len(set.Bundles)),
			attribute.String("messaging.entity", receiverTopicOrQueue.Name),
		),
	)
	defer span.End()

	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to serialize bundle set")
		return err
	}

	span.SetAttributes(
		attribute.Int("payload.size_bytes", len(jsonData)),
	)

	// TODO: Remove support for receiverTopicOrQueue, and then this if-guard
	if taskAcceptedBundleEndpoint != "" {
		span.SetAttributes(
			attribute.String("transport.type", "http"),
			attribute.String("http.url", taskAcceptedBundleEndpoint),
		)

		log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to HTTP endpoint (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
		httpResponse, err := http.Post(taskAcceptedBundleEndpoint, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to send HTTP request")
			log.Ctx(ctx).Warn().Err(err).Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
			return errors.Wrap(err, "failed to send task to endpoint")
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
			return errors.Errorf("failed to send task to endpoint, status code: %d", httpResponse.StatusCode)
		}
	} else {
		span.SetAttributes(
			attribute.String("transport.type", "message_broker"),
		)

		log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to message broker with messages %s", set.task, jsonData)
		msg := &messaging.Message{
			Body:          jsonData,
			ContentType:   "application/json",
			CorrelationID: &set.Id,
		}
		err = messageBroker.SendMessage(ctx, receiverTopicOrQueue, msg)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to send message to broker")
			log.Ctx(ctx).Warn().Msgf("Sending set for task (ref=%s) to message broker failed, error: %s", set.task, err.Error())
			return errors.Wrap(err, "failed to send task to message broker")
		}
	}
	log.Ctx(ctx).Info().Msgf("Notified EHR of accepted Task with bundle (ref=%s)", set.task)
	return nil
}
