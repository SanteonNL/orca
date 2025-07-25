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
	return n.eventManager.Notify(ctx, TaskAcceptedEvent{
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	})
}

func (n *notifier) start(receiverTopicOrQueue messaging.Entity, taskAcceptedBundleEndpoint string) error {
	// TODO: add sync call here to the API of DataHub for enrolling a patient
	// IF the call fails here we want to create a subtask to let the hospital know that the enrollment failed
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
		//TODO: If this fails, we should create a subtask to let the hospital know that the enrollment failed and the datahub returns a
		// questionnaire URL to fill in the missing data
		// cpsClient.CreateWithContext(ctx, "Task", )
		return sendBundle(ctx, receiverTopicOrQueue, taskAcceptedBundleEndpoint, *bundles, n.broker)
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, receiverTopicOrQueue messaging.Entity, taskAcceptedBundleEndpoint string, set BundleSet, messageBroker messaging.Broker) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	// TODO: Remove support for receiverTopicOrQueue, and then this if-guard
	if taskAcceptedBundleEndpoint != "" {
		log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
		httpResponse, err := http.Post(taskAcceptedBundleEndpoint, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Ctx(ctx).Warn().Err(err).Msgf("Sending set for task (ref=%s) to HTTP endpoint failed (endpoint=%s)", set.task, taskAcceptedBundleEndpoint)
			return errors.Wrap(err, "failed to send task to endpoint")
		}
		defer httpResponse.Body.Close()
		if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
			return errors.Errorf("failed to send task to endpoint, status code: %d", httpResponse.StatusCode)
		}
	} else {
		log.Ctx(ctx).Info().Msgf("Sending set for task (ref=%s) to message broker with messages %s", set.task, jsonData)
		msg := &messaging.Message{
			Body:          jsonData,
			ContentType:   "application/json",
			CorrelationID: &set.Id,
		}
		// TODO: Remove this and send the message directly with an API call to DataHub
		err = messageBroker.SendMessage(ctx, receiverTopicOrQueue, msg)
		if err != nil {
			log.Ctx(ctx).Warn().Msgf("Sending set for task (ref=%s) to message broker failed, error: %s", set.task, err.Error())
			return errors.Wrap(err, "failed to send task to message broker")
		}
	}
	log.Ctx(ctx).Info().Msgf("Notified EHR of accepted Task with bundle (ref=%s)", set.task)
	return nil
}
