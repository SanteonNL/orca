//go:generate mockgen -destination=./manager_mock.go -package=subscriptions -source=manager.go
package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

const tracerName = "careplanservice.subscriptions"

var timeFunc = time.Now

type Manager interface {
	Notify(ctx context.Context, resource interface{}) error
}

func NewManager(cpsBaseURLFunc func(tenants.Properties) *url.URL, tenants tenants.Config, channels ChannelFactory, messageBroker messaging.Broker) (*RetryableManager, error) {
	mgr := &RetryableManager{
		cpsBaseURLFunc: cpsBaseURLFunc,
		tenants:        tenants,
		channels:       channels,
		messageBroker:  messageBroker,
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
	cpsBaseURLFunc func(tenants.Properties) *url.URL
	tenants        tenants.Config
	channels       ChannelFactory
	messageBroker  messaging.Broker
}

type NotificationEvent struct {
	Subscriber fhir.Identifier `json:"subscriber"`
	Focus      fhir.Reference  `json:"focus"`
	TenantID   string          `json:"tenant_id"`
}

func (r RetryableManager) Notify(ctx context.Context, resource interface{}) error {
	start := time.Now()
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"RetryableManager.Notify",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("notification.resource_type", coolfhir.ResourceType(resource)),
			attribute.String("operation.name", "NotifySubscribers"),
		),
	)
	defer span.End()

	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tenant from context")
		return err
	}

	span.SetAttributes(
		attribute.String("tenant.id", tenant.ID),
	)

	var focus fhir.Reference
	var subscribers []fhir.Identifier
	resourceType := coolfhir.ResourceType(resource)

	switch resourceType {
	case "Task":
		task := resource.(*fhir.Task)
		focus = fhir.Reference{
			Reference: to.Ptr("Task/" + *task.Id),
			Type:      to.Ptr("Task"),
		}

		span.SetAttributes(attribute.String("fhir.resource_id", *task.Id))

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

		span.SetAttributes(attribute.String("fhir.resource_id", *careTeam.Id))

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

		span.SetAttributes(attribute.String("fhir.resource_id", *carePlan.Id))

		careTeam, err := coolfhir.CareTeamFromCarePlan(carePlan)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to read CareTeam from CarePlan")
			log.Ctx(ctx).Err(err).Msg("failed to read CareTeam in CarePlan")
			return nil
		}

		for _, participant := range careTeam.Participant {
			if coolfhir.IsLogicalIdentifier(participant.Member.Identifier) {
				subscribers = append(subscribers, *participant.Member.Identifier)
			}
		}
	default:
		err := fmt.Errorf("subscription manager does not support notifying for resource type: %T", resource)
		span.RecordError(err)
		span.SetStatus(codes.Error, "unsupported resource type")
		return err
	}

	span.SetAttributes(
		attribute.String("fhir.focus_reference", *focus.Reference),
		attribute.Int("notification.subscriber_count", len(subscribers)),
	)

	log.Ctx(ctx).Info().Msgf("Notifying %d subscriber(s) for update on resource: %s", len(subscribers), *focus.Reference)

	var errs []error
	successCount := 0
	for _, subscriber := range subscribers {
		data, _ := json.Marshal(NotificationEvent{
			Subscriber: subscriber,
			Focus:      focus,
			TenantID:   tenant.ID,
		})
		if err := r.messageBroker.SendMessage(ctx, SendNotificationQueue, &messaging.Message{
			Body:        data,
			ContentType: "application/json",
		}); err != nil {
			errs = append(errs, fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(subscriber), err))
		} else {
			successCount++
		}
	}

	span.SetAttributes(
		attribute.Int("notification.success_count", successCount),
		attribute.Int("notification.error_count", len(errs)),
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)

	if len(errs) > 0 {
		joinedErr := fmt.Errorf("couldn't notify all subscribers (this is non-recoverable!): %w", errors.Join(errs...))
		span.RecordError(joinedErr)
		span.SetStatus(codes.Error, "partial notification failure")
		return joinedErr
	} else {
		span.SetStatus(codes.Ok, "")
		return nil
	}
}

func (r RetryableManager) receiveMessage(ctx context.Context, message messaging.Message) error {
	start := time.Now()
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(
		ctx,
		"RetryableManager.receiveMessage",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.operation", "receive"),
			attribute.String("messaging.destination.name", SendNotificationQueue.Name),
			attribute.String("operation.name", "ProcessNotification"),
		),
	)
	defer span.End()

	var evt NotificationEvent
	if err := json.Unmarshal(message.Body, &evt); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to unmarshal notification event")
		return fmt.Errorf("failed to unmarshal message into %T: %w", evt, err)
	}

	// Add notification event details to span
	span.SetAttributes(
		attribute.String("tenant.id", evt.TenantID),
		attribute.String("notification.focus_reference", *evt.Focus.Reference),
		attribute.String("notification.focus_type", *evt.Focus.Type),
		attribute.String("notification.subscriber", coolfhir.ToString(evt.Subscriber)),
	)

	// Enrich context with the correct tenant
	tenant, err := r.tenants.Get(evt.TenantID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get tenant")
		return fmt.Errorf("get tenant %s: %w", evt.TenantID, err)
	}
	ctx = tenants.WithTenant(ctx, *tenant)

	span.SetAttributes(attribute.String("tenant.id", tenant.ID))

	channel, err := r.channels.Create(ctx, evt.Subscriber)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create notification channel")
		return fmt.Errorf("notification-channel for subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err)
	}

	// TODO: refer to stored subscription
	subscription := fhir.Reference{
		Reference: to.Ptr("Subscription/" + uuid.NewString()),
	}

	span.SetAttributes(attribute.String("fhir.subscription_id", *subscription.Reference))

	// TODO: Read event number from store
	// TODO: Do we need an audit event for subscription notifications?
	cpsBaseURL := r.cpsBaseURLFunc(*tenant)
	notification := coolfhir.CreateSubscriptionNotification(cpsBaseURL, timeFunc(), subscription, 0, evt.Focus)

	span.SetAttributes(attribute.String("notification.cps_base_url", cpsBaseURL.String()))

	if err = channel.Notify(ctx, notification); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to send notification to channel")
		return fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err)
	}

	span.SetAttributes(
		attribute.String("notification.status", "delivered"),
		attribute.Int64("operation.duration_ms", time.Since(start).Milliseconds()),
	)
	span.SetStatus(codes.Ok, "")
	return nil
}
