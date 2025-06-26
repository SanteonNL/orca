//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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
func NewNotifier(eventManager events.Manager, messageBroker messaging.Broker, receiverTopicOrQueue messaging.Entity, fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)) (Notifier, error) {
	n := &notifier{
		eventManager:      eventManager,
		broker:            messageBroker,
		fhirClientFactory: fhirClientFactory,
	}
	if err := n.start(receiverTopicOrQueue); err != nil {
		return nil, err
	}
	return n, nil
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a message broker.
func (n *notifier) NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error {
	return n.eventManager.Notify(ctx, TaskAcceptedEvent{
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	})
}

func (n *notifier) start(receiverTopicOrQueue messaging.Entity) error {
	return n.eventManager.Subscribe(TaskAcceptedEvent{}, func(ctx context.Context, rawEvent events.Type) error {
		event := rawEvent.(*TaskAcceptedEvent)
		fhirBaseURL, err := url.Parse(event.FHIRBaseURL)
		if err != nil {
			return err
		}

		cpsClient, _, err := n.fhirClientFactory(ctx, fhirBaseURL)
		if err != nil {
			return errors.Wrap(err, "failed to create FHIR client for invoking CarePlanService")
		}

		bundles, err := TaskNotificationBundleSet(ctx, cpsClient, *event.Task.Id)
		if err != nil {
			return errors.Wrap(err, "failed to create task notification bundle")
		}
		log.Ctx(ctx).Info().Msgf("Sending set for task notifier started")
		return sendBundle(ctx, receiverTopicOrQueue, *bundles, n.broker)
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, receiverTopicOrQueue messaging.Entity, set BundleSet, messageBroker messaging.Broker) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to message broker with messages %s", set.task, jsonData)
	msg := &messaging.Message{
		Body:          jsonData,
		ContentType:   "application/json",
		CorrelationID: &set.Id,
	}
	err = messageBroker.SendMessage(ctx, receiverTopicOrQueue, msg)
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("Sending set for task (ref=%s) to message broker failed, error: %s", set.task, err.Error())
		return errors.Wrap(err, "failed to send task to message broker")
	}

	log.Ctx(ctx).Info().Msgf("Notified EHR of accepted Task with bundle (ref=%s)", set.task)
	return nil
}
