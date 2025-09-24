package messaging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

var _ Broker = &AzureServiceBusBroker{}

// AzureServiceBusConfig holds the configuration for connecting to and interacting with a AzureServiceBus instance.
type AzureServiceBusConfig struct {
	Hostname         string `koanf:"hostname"`
	ConnectionString string `koanf:"connectionstring" description:"This is the connection string for connecting to AzureServiceBus."`
}

func (a AzureServiceBusConfig) Enabled() bool {
	return a.Hostname != "" || a.ConnectionString != ""
}

func newAzureServiceBusBroker(conf AzureServiceBusConfig, entities []Entity, entityPrefix string) (*AzureServiceBusBroker, error) {
	var client *azservicebus.Client
	var err error
	if conf.ConnectionString != "" {
		client, err = azservicebus.NewClientFromConnectionString(conf.ConnectionString, nil)
	} else if conf.Hostname != "" {
		var cred *azidentity.DefaultAzureCredential
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		client, err = azservicebus.NewClient(conf.Hostname, cred, nil)
	} else {
		return nil, errors.New("configuration is missing hostname or connection string")
	}
	if err != nil {
		return nil, err
	}
	senders := map[string]*azservicebus.Sender{}
	for _, topic := range entities {
		sender, err := client.NewSender(topic.FullName(entityPrefix), nil)
		if err != nil {
			return nil, fmt.Errorf("create topic (name=%s)", topic.FullName(entityPrefix))
		}
		senders[topic.Name] = sender
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AzureServiceBusBroker{
		client:       client,
		senders:      senders,
		entityPrefix: entityPrefix,
		ctx:          ctx,
		ctxCancel:    cancel,
	}, nil
}

// AzureServiceBusBroker is an implementation of the messageBroker interface for interacting with Azure Service Bus.
// It wraps an azservicebus.Sender for sending messages to a specific Service Bus topic.
// This struct provides methods to send messages and close the underlying Service Bus connection.
type AzureServiceBusBroker struct {
	senders      map[string]*azservicebus.Sender
	senderLock   sync.RWMutex
	client       *azservicebus.Client
	entityPrefix string
	ctx          context.Context
	ctxCancel    context.CancelFunc
	receivers    sync.WaitGroup
}

// Close releases the underlying resources associated with the AzureServiceBusBroker instance.
func (c *AzureServiceBusBroker) Close(ctx context.Context) error {
	slog.DebugContext(ctx, "AzureServiceBus: closing...")
	c.senderLock.Lock()
	defer c.senderLock.Unlock()

	// Wait for all receivers to finish before closing the client.
	slog.DebugContext(ctx, "AzureServiceBus: waiting for all receivers to close")
	c.ctxCancel()
	c.receivers.Wait()

	// Collect all close() errors from senders, receivers and client, then return them as a whole.
	slog.DebugContext(ctx, "AzureServiceBus: waiting for all senders to close")
	var errs []error
	for topic, sender := range c.senders {
		if err := sender.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close sender (topic=%s): %w", topic, err))
		}
		delete(c.senders, topic)
	}
	slog.DebugContext(ctx, "AzureServiceBus: finally, closing client")
	if err := c.client.Close(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to close client: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{
			errors.New("azure service bus: close() failures")},
			errs...,
		)...)
	}
	slog.DebugContext(ctx, "AzureServiceBus: closed")
	return nil
}

func (c *AzureServiceBusBroker) ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error {
	fullName := queue.FullName(c.entityPrefix)
	receiver, err := c.client.NewReceiverForQueue(fullName, &azservicebus.ReceiverOptions{})
	if err != nil {
		return fmt.Errorf("AzureServiceBus: create receiver (queue=%s)", fullName)
	}
	c.receive(receiver, fullName, handler)
	return nil
}

func (c *AzureServiceBusBroker) receive(receiver *azservicebus.Receiver, fullName string, handler func(context.Context, Message) error) {
	c.receivers.Add(1)
	go func() {
		defer c.receivers.Done()
		for c.ctx.Err() == nil {
			messages, err := receiver.ReceiveMessages(c.ctx, 1, &azservicebus.ReceiveMessagesOptions{})
			if err != nil || len(messages) == 0 {
				const backoffTime = time.Minute
				if !errors.Is(err, context.Canceled) {
					slog.ErrorContext(
						c.ctx,
						"AzureServiceBus: receive message failed, backing off",
						slog.String("source", fullName),
						slog.String("error", err.Error()),
						slog.Duration("backoff_time", backoffTime),
					)
				}
				// Sleep for a minute before retrying, to avoid spamming the logs.
				// But, the server might be instructed to shut down in the meantime, and we don't want the sleep to block the shutdown.
				// So, use a select to also listen for shutdown.
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(backoffTime):
				}
				continue
			}
			azMessage := messages[0]
			message := Message{
				Body:          azMessage.Body,
				CorrelationID: azMessage.CorrelationID,
			}
			if azMessage.ContentType != nil {
				message.ContentType = *azMessage.ContentType
			}
			if err := handler(c.ctx, message); err != nil {
				slog.ErrorContext(
					c.ctx,
					"AzureServiceBus: message handler failed, message will be sent to DLQ",
					slog.String("source", fullName),
					slog.String("error", err.Error()),
				)
				if err := receiver.AbandonMessage(c.ctx, azMessage, &azservicebus.AbandonMessageOptions{
					PropertiesToModify: map[string]any{
						"deliveryfailure-" + strconv.Itoa(int(azMessage.DeliveryCount)): err.Error(),
					},
				}); err != nil {
					slog.ErrorContext(
						c.ctx,
						"AzureServiceBus: abandon message (for redelivery) failed",
						slog.String("source", fullName),
						slog.String("error", err.Error()),
					)
				}
			} else {
				if err := receiver.CompleteMessage(c.ctx, azMessage, &azservicebus.CompleteMessageOptions{}); err != nil {
					slog.ErrorContext(
						c.ctx,
						"AzureServiceBus: complete message failed",
						slog.String("source", fullName),
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}()
}

// SendMessage sends a message to the associated Azure Service Bus senders client. It returns an error if the operation fails.
func (c *AzureServiceBusBroker) SendMessage(ctx context.Context, queueOrTopic Entity, message *Message) error {
	c.senderLock.RLock()
	defer c.senderLock.RUnlock()
	sender, ok := c.senders[queueOrTopic.Name]
	if !ok {
		return fmt.Errorf("AzureServiceBus: sender not found (entity=%s)", queueOrTopic.Name)
	}
	serviceBusMsg := &azservicebus.Message{
		Body:          message.Body,
		ContentType:   &message.ContentType,
		CorrelationID: message.CorrelationID,
	}
	return sender.SendMessage(ctx, serviceBusMsg, nil)
}
