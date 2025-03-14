//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

var TaskEngineTaskAcceptedQueueName = messaging.Topic{
	Name:   "orca.taskengine.task-accepted",
	Prefix: true,
}

// Notifier is an interface for sending notifications regarding task acceptance within a FHIR-based system.
type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error
}

// notifier is a type that uses a ServiceBusClient to send messages to a message broker.
type notifier struct {
	broker            messaging.Broker
	fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)
}

type acceptedTaskEvent struct {
	FHIRBaseURL string    `json:"fhirBaseURL"`
	Task        fhir.Task `json:"task"`
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(messageBroker messaging.Broker, topic messaging.Topic, fhirClientFactory func(ctx context.Context, fhirBaseURL *url.URL) (fhirclient.Client, *http.Client, error)) (Notifier, error) {
	n := &notifier{
		broker:            messageBroker,
		fhirClientFactory: fhirClientFactory,
	}
	if err := n.start(topic); err != nil {
		return nil, err
	}
	return n, nil
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a message broker.
func (n *notifier) NotifyTaskAccepted(ctx context.Context, fhirBaseURL string, task *fhir.Task) error {
	payload, _ := json.Marshal(acceptedTaskEvent{
		FHIRBaseURL: fhirBaseURL,
		Task:        *task,
	})
	return n.broker.SendMessage(ctx, TaskEngineTaskAcceptedQueueName, &messaging.Message{
		Body:        payload,
		ContentType: "application/json",
	})
}

func (n *notifier) start(sendToTopic messaging.Topic) error {
	return n.broker.Receive(TaskEngineTaskAcceptedQueueName, func(ctx context.Context, message messaging.Message) error {
		var event acceptedTaskEvent
		if err := json.Unmarshal(message.Body, &event); err != nil {
			return fmt.Errorf("failed to unmarshal message into %T: %w", event, err)
		}
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
		return sendBundle(ctx, sendToTopic, *bundles, n.broker)
	})
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, topic messaging.Topic, set BundleSet, messageBroker messaging.Broker) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Ctx(ctx).Debug().Msgf("Sending set for task (ref=%s) to message broker", set.task)
	msg := &messaging.Message{
		Body:          jsonData,
		ContentType:   "application/json",
		CorrelationID: &set.Id,
	}
	err = messageBroker.SendMessage(ctx, topic, msg)
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("Sending set for task (ref=%s) to message broker failed, error: %s", set.task, err.Error())
		return errors.Wrap(err, "failed to send task to message broker")
	}

	log.Ctx(ctx).Debug().Msgf("Successfully sent task (ref=%s) to message broker", set.task)
	return nil
}
