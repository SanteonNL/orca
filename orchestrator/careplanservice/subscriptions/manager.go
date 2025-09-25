//go:generate mockgen -destination=./manager_mock.go -package=subscriptions -source=manager.go
package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/lib/debug"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	baseotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

var SendNotificationQueue = messaging.Entity{
	Name:   "orca.subscriptionmgr.notification",
	Prefix: true,
}

var tracer = baseotel.Tracer("careplanservice.subscriptions")

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
	if err := messageBroker.ReceiveFromQueue(SendNotificationQueue, mgr.tryNotify); err != nil {
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
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.String("notification.resource_type", coolfhir.ResourceType(resource)),
		),
	)
	defer span.End()

	tenant, err := tenants.FromContext(ctx)
	if err != nil {
		return otel.Error(span, err, "failed to get tenant from context")
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

		span.SetAttributes(attribute.String(otel.FHIRResourceID, *task.Id))

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

		span.SetAttributes(attribute.String(otel.FHIRResourceID, *careTeam.Id))

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

		span.SetAttributes(attribute.String(otel.FHIRResourceID, *carePlan.Id))

		careTeam, err := coolfhir.CareTeamFromCarePlan(carePlan)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to read CareTeam from CarePlan",
				slog.String(logging.FieldResourceType, resourceType),
				slog.String(logging.FieldResourceID, *carePlan.Id),
				slog.String(logging.FieldError, otel.Error(span, err, "failed to read CarePlan").Error()),
			)
			return nil
		}

		for _, participant := range careTeam.Participant {
			if coolfhir.IsLogicalIdentifier(participant.Member.Identifier) {
				subscribers = append(subscribers, *participant.Member.Identifier)
			}
		}
	default:
		return otel.Error(span, fmt.Errorf("subscription manager does not support notifying for resource type: %s", coolfhir.ResourceType(resource)), "unsupported resource type")
	}

	span.SetAttributes(
		attribute.String("fhir.focus_reference", *focus.Reference),
		attribute.Int("notification.subscriber_count", len(subscribers)),
	)

	slog.InfoContext(
		ctx,
		"Notifying subscriber(s) for update on resource",
		slog.Int("subscriber_count", len(subscribers)),
		slog.String(logging.FieldResourceReference, *focus.Reference),
		slog.String(logging.FieldResourceType, resourceType),
	)

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
	)

	if len(errs) > 0 {
		joinedErr := fmt.Errorf("couldn't notify all subscribers (this is non-recoverable!): %w", errors.Join(errs...))
		return otel.Error(span, joinedErr, "partial notification failure")
	} else {
		span.SetStatus(codes.Ok, "")
		return nil
	}
}

func (r RetryableManager) tryNotify(ctx context.Context, message messaging.Message) error {
	ctx, span := tracer.Start(
		ctx,
		debug.GetFullCallerName(),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.operation", "receive"),
			attribute.String("messaging.destination.name", SendNotificationQueue.Name),
		),
	)
	defer span.End()

	var evt NotificationEvent
	if err := json.Unmarshal(message.Body, &evt); err != nil {
		return otel.Error(span, fmt.Errorf("failed to unmarshal message into %T: %w", evt, err), "failed to unmarshal notification event")
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
		return otel.Error(span, fmt.Errorf("get tenant %s: %w", evt.TenantID, err), "failed to get tenant")
	}
	ctx = tenants.WithTenant(ctx, *tenant)

	span.SetAttributes(attribute.String("tenant.id", tenant.ID))

	channel, err := r.channels.Create(ctx, evt.Subscriber)
	if err != nil {
		return otel.Error(span, fmt.Errorf("notification-channel for subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err), "failed to create notification channel")
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
		return otel.Error(span, fmt.Errorf("notify subscriber %s: %w", coolfhir.ToString(evt.Subscriber), err), "failed to send notification to channel")
	}

	span.SetAttributes(
		attribute.String("notification.status", "delivered"),
	)
	span.SetStatus(codes.Ok, "")
	return nil
}
