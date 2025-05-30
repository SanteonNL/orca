//go:generate mockgen -destination=./manager_mock.go -package=subscriptions -source=manager.go
package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"net/url"
	"time"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var SendNotificationQueue = messaging.Entity{
	Name:   "orca.subscriptionmgr.notification",
	Prefix: true,
}

var timeFunc = time.Now

type Manager interface {
	Notify(ctx context.Context, resource interface{}) error
}

func NewManager(fhirBaseURL *url.URL, channels ChannelFactory, messageBroker messaging.Broker) (*RetryableManager, error) {
	mgr := &RetryableManager{
		FhirBaseURL:   fhirBaseURL,
		Channels:      channels,
		MessageBroker: messageBroker,
	}
	if err := messageBroker.ReceiveFromQueue(SendNotificationQueue, mgr.receiveMessage); err != nil {
		return nil, err
	}
	return mgr, nil
}

var _ Manager = RetryableManager{}

// RetryableManager is a Manager derives Subscriptions from the properties of FHIR resource
// that triggered the notification:
// - Task: it notifies the Task filler and owner
// - CareTeam: it notifies all participants
// TODO: It does not yet store the subscription notifications in the FHIR store, which is required to support monotonically increasing event numbers.
type RetryableManager struct {
	FhirBaseURL   *url.URL
	Channels      ChannelFactory
	MessageBroker messaging.Broker
}

type NotificationEvent struct {
	Subscriber fhir.Identifier `json:"subscriber"`
	Focus      fhir.Reference  `json:"focus"`
}

func (r RetryableManager) Notify(ctx context.Context, resource interface{}) error {
	var focus fhir.Reference
	var subscribers []fhir.Identifier
	switch coolfhir.ResourceType(resource) {
	case "Task":
		task := resource.(*fhir.Task)
		focus = fhir.Reference{
			Reference: to.Ptr("Task/" + *task.Id),
			Type:      to.Ptr("Task"),
		}
		if task.Owner != nil {
			if coolfhir.IsLogicalIdentifier(task.Owner.Identifier) {
				subscribers = append(subscribers, *task.Owner.Identifier)
			}
		}

		if task.Requester != nil {
			if coolfhir.IsLogicalIdentifier(task.Requester.Identifier) {
				subscribers = append(subscribers, *task.Requester.Identifier)
			}
		}
	case "CareTeam":
		careTeam := resource.(*fhir.CareTeam)
		focus = fhir.Reference{
			Reference: to.Ptr("CareTeam/" + *careTeam.Id),
			Type:      to.Ptr("CareTeam"),
		}

		for _, participant := range careTeam.Participant {
			if coolfhir.IsLogicalIdentifier(participant.Member.Identifier) {
				subscribers = append(subscribers, *participant.Member.Identifier)
			}
		}
	case "CarePlan":
		carePlan := resource.(*fhir.CarePlan)
		focus = fhir.Reference{
			Reference: to.Ptr("CarePlan/" + *carePlan.Id),
			Type:      to.Ptr("CarePlan"),
		}

		careTeam, err := coolfhir.CareTeamFromCarePlan(carePlan)
		if err != nil {
			log.Ctx(ctx).Err(err).Msg("failed to read CareTeam in CarePlan")
			return nil
		}

		for _, participant := range careTeam.Participant {
			if coolfhir.IsLogicalIdentifier(participant.Member.Identifier) {
				subscribers = append(subscribers, *participant.Member.Identifier)
			}
		}
	default:
		return fmt.Errorf("subscription manager does not support notifying for resource type: %T", resource)
	}

	log.Ctx(ctx).Info().Msgf("Notifying %d subscriber(s) for update on resource: %s", len(subscribers), *focus.Reference)

	var errs []error
	for _, subscriber := range subscribers {
		data, _ := json.Marshal(NotificationEvent{
			Subscriber: subscriber,
			Focus:      focus,
		})
		if err := r.MessageBroker.SendMessage(ctx, SendNotificationQueue, &messaging.Message{
			Body:        data,
			ContentType: "application/json",
		}); err != nil {
			errs = append(errs, fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(subscriber), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("couldn't notify all subscribers (this is non-recoverable!): %w", errors.Join(errs...))
	} else {
		return nil
	}
}

func (r RetryableManager) receiveMessage(ctx context.Context, message messaging.Message) error {
	var evt NotificationEvent
	if err := json.Unmarshal(message.Body, &evt); err != nil {
		return fmt.Errorf("failed to unmarshal message into %T: %w", evt, err)
	}

	channel, err := r.Channels.Create(ctx, evt.Subscriber)
	if err != nil {
		return fmt.Errorf("notification-channel for subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err)
	}
	// TODO: refer to stored subscription
	subscription := fhir.Reference{
		Reference: to.Ptr("Subscription/" + uuid.NewString()),
	}
	// TODO: Read event number from store
	// TODO: Do we need an audit event for subscription notifications?
	notification := coolfhir.CreateSubscriptionNotification(r.FhirBaseURL, timeFunc(), subscription, 0, evt.Focus)
	if err = channel.Notify(ctx, notification); err != nil {
		return fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err)
	}
	return nil
}
