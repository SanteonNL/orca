package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"net/url"
	"sync"
	"time"
)

var timeFunc = time.Now

type Manager interface {
	Notify(ctx context.Context, resource interface{}) error
}

var _ Manager = DerivingManager{}

// DerivingManager is a Manager derives Subscriptions from the properties of FHIR resource
// that triggered the notification:
// - Task: it notifies the Task filler and owner
// - CareTeam: it notifies all participants
// TODO: It does not yet store the subscription notifications in the FHIR store, which is required to support monotonically increasing event numbers.
type DerivingManager struct {
	FhirBaseURL *url.URL
	Channels    ChannelFactory
}

func (r DerivingManager) Notify(ctx context.Context, resource interface{}) error {
	var focus fhir.Reference
	var subscribers []fhir.Identifier
	switch coolfhir.ResourceType(resource) {
	case "Task":
		task := resource.(*fhir.Task)
		focus = fhir.Reference{
			Reference: to.Ptr("Task/" + *task.Id),
			Type:      to.Ptr("Task"),
		}
		if coolfhir.IsLogicalReference(task.Owner) {
			subscribers = append(subscribers, *task.Owner.Identifier)
		}
		if coolfhir.IsLogicalReference(task.Requester) {
			subscribers = append(subscribers, *task.Requester.Identifier)
		}
	case "CareTeam":
		careTeam := resource.(*fhir.CareTeam)
		focus = fhir.Reference{
			Reference: to.Ptr("CareTeam/" + *careTeam.Id),
		}
		for _, participant := range careTeam.Participant {
			if coolfhir.IsLogicalReference(participant.Member) {
				subscribers = append(subscribers, *participant.Member.Identifier)
			}
		}
	default:
		return fmt.Errorf("subscription manager does not support notifying for resource type: %T", resource)
	}

	return r.notifyAll(ctx, timeFunc(), focus, subscribers)
}

func (r DerivingManager) notifyAll(ctx context.Context, instant time.Time, focus fhir.Reference, subscribers []fhir.Identifier) error {
	errs := make(chan error, len(subscribers))
	notifyFinished := &sync.WaitGroup{}
	for _, subscriber := range subscribers {
		notifyFinished.Add(1)
		go func(subscriber fhir.Identifier) {
			defer notifyFinished.Done()
			channel, err := r.Channels.Create(ctx, subscriber)
			if err != nil {
				errs <- fmt.Errorf("notification-channel for subscriber %s: %w", coolfhir.ToString(subscriber), err)
				return
			}
			// TODO: refer to stored subscription
			subscription := fhir.Reference{
				Reference: to.Ptr("Subscription/" + uuid.NewString()),
			}
			// TODO: Read event number from store
			notification := coolfhir.CreateSubscriptionNotification(r.FhirBaseURL, instant, subscription, 0, focus)
			if err = channel.Notify(ctx, notification); err != nil {
				errs <- fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(subscriber), err)
			}
		}(subscriber)
	}
	notifyFinished.Wait()
	var result []error
	for i := 0; i < len(errs); i++ {
		result = append(result, <-errs)
	}
	if len(result) > 0 {
		return errors.Join(result...)
	}
	return nil
}
