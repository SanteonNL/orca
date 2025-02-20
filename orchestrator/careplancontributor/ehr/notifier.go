//go:generate mockgen -destination=./notifier_mock.go -package=ehr -source=notifier.go
package ehr

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Notifier is an interface for sending notifications regarding task acceptance within a FHIR-based system.
type Notifier interface {
	NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error
}

// kafkaNotifier is a type that uses a ServiceBusClient to send messages to a Kafka service.
type kafkaNotifier struct {
	kafkaClient ServiceBusClient
}

// NewNotifier creates and returns a Notifier implementation using the provided ServiceBusClient for message handling.
func NewNotifier(kafkaClient ServiceBusClient) Notifier {
	return &kafkaNotifier{kafkaClient}
}

// NotifyTaskAccepted sends notification data comprehensively related to a specific FHIR Task to a Kafka service bus.
func (n *kafkaNotifier) NotifyTaskAccepted(ctx context.Context, cpsClient fhirclient.Client, task *fhir.Task) error {

	bundles, err := TaskNotificationBundleSet(ctx, cpsClient, *task.Id)
	if err != nil {
		return errors.Wrap(err, "failed to create task notification bundle")
	}
	return sendBundle(ctx, *bundles, n.kafkaClient)
}

// sendBundle sends a serialized BundleSet to a Service Bus using the provided ServiceBusClient.
// It logs the process and errors during submission while wrapping and returning them.
// Returns an error if serialization or message submission fails.
func sendBundle(ctx context.Context, set BundleSet, kafkaClient ServiceBusClient) error {
	jsonData, err := json.MarshalIndent(set, "", "\t")
	if err != nil {
		return err
	}
	log.Ctx(ctx).Debug().Msgf("Sending set for task (ref=%s) to ServiceBus", set.task)
	err = kafkaClient.SubmitMessage(ctx, set.Id, string(jsonData))
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("Sending set for task (ref=%s) to ServiceBus failed, error: %s", set.task, err.Error())
		return errors.Wrap(err, "failed to send task to ServiceBus")
	}

	log.Ctx(ctx).Debug().Msgf("Successfully sent task (ref=%s) to ServiceBus", set.task)
	return nil
}
