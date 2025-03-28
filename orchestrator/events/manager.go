package events

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/messaging"
)

type Type interface {
	Entity() messaging.Entity
	Instance() Type
}

type Manager interface {
	Subscribe(eventType Type, handler HandleFunc) error
	Notify(ctx context.Context, instance Type) error
	HasSubscribers(eventType Type) bool
}

func NewManager(messageBroker messaging.Broker) *DefaultManager {
	return &DefaultManager{
		messageBroker: messageBroker,
		subscribers:   map[string]bool{},
	}
}

var _ Manager = &DefaultManager{}

type DefaultManager struct {
	messageBroker messaging.Broker
	subscribers   map[string]bool
}

func (d DefaultManager) HasSubscribers(eventType Type) bool {
	_, ok := d.subscribers[eventType.Entity().Name]
	return ok
}

func (d DefaultManager) Subscribe(eventType Type, handler HandleFunc) error {
	d.subscribers[eventType.Entity().Name] = true
	return d.messageBroker.ReceiveFromQueue(eventType.Entity(), func(ctx context.Context, message messaging.Message) error {
		event := eventType.Instance()
		if err := json.Unmarshal(message.Body, event); err != nil {
			return fmt.Errorf("event %T unmarshal: %w", eventType, err)
		}
		err := handler(ctx, (event).(Type))
		if err != nil {
			return fmt.Errorf("event handler %T: %w", event, err)
		}
		return nil
	})
}

func (d DefaultManager) Notify(ctx context.Context, instance Type) error {
	messageData, err := json.Marshal(instance)
	if err != nil {
		return err
	}
	err = d.messageBroker.SendMessage(ctx, instance.Entity(), &messaging.Message{
		Body:        messageData,
		ContentType: "application/fhir+json",
	})
	if err != nil {
		return fmt.Errorf("event send %T: %w", instance, err)
	}
	return nil
}

type Handler interface {
	Handle(ctx context.Context, event Type) error
}

type HandleFunc func(ctx context.Context, event Type) error
