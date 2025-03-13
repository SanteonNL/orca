package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

// AzureServiceBusConfig holds the configuration for connecting to and interacting with a AzureServiceBus instance.
type AzureServiceBusConfig struct {
	Hostname         string `koanf:"hostname"`
	ConnectionString string `koanf:"connectionstring" description:"This is the connection string for connecting to AzureServiceBus."`
}

func newAzureServiceBusBroker(conf AzureServiceBusConfig, topics []string, topicPrefix string) (*AzureServiceBusBroker, error) {
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
	for _, topic := range topics {
		sender, err := client.NewSender(topic, nil)
		if err != nil {
			return nil, fmt.Errorf("create topic (name=%s)", topic)
		}
		senders[topic] = sender
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AzureServiceBusBroker{
		client:      client,
		senders:     senders,
		topicPrefix: topicPrefix,
		ctx:         ctx,
		ctxCancel:   cancel,
	}, nil
}

// AzureServiceBusBroker is an implementation of the MessageBroker interface for interacting with Azure Service Bus.
// It wraps an azservicebus.Sender for sending messages to a specific Service Bus topic.
// This struct provides methods to send messages and close the underlying Service Bus connection.
type AzureServiceBusBroker struct {
	senders     map[string]*azservicebus.Sender
	senderLock  sync.RWMutex
	client      *azservicebus.Client
	topicPrefix string
	ctx         context.Context
	ctxCancel   context.CancelFunc
	receivers   sync.WaitGroup
}

// Close releases the underlying resources associated with the AzureServiceBusBroker instance.
func (c *AzureServiceBusBroker) Close(ctx context.Context) error {
	log.Ctx(ctx).Debug().Msg("AzureServiceBus: closing...")
	c.senderLock.Lock()
	defer c.senderLock.Unlock()

	// Wait for all receivers to finish before closing the client.
	log.Ctx(ctx).Debug().Msg("AzureServiceBus: waiting for all receivers to close")
	c.ctxCancel()
	c.receivers.Wait()

	// Collect all close() errors from senders, receivers and client, then return them as a whole.
	log.Ctx(ctx).Debug().Msg("AzureServiceBus: waiting for all senders to close")
	var errs []error
	for topic, sender := range c.senders {
		if err := sender.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close sender (topic=%s): %w", topic, err))
		}
		delete(c.senders, topic)
	}
	log.Ctx(ctx).Debug().Msg("AzureServiceBus: finally, closing client")
	if err := c.client.Close(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to close client: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{
			errors.New("azure service bus: close() failures")},
			errs...,
		)...)
	}
	log.Ctx(ctx).Debug().Msg("AzureServiceBus: closed")
	return nil
}

func (c *AzureServiceBusBroker) Receive(queue string, handler func(context.Context, Message) error) error {
	receiver, err := c.client.NewReceiverForQueue(c.topicName(queue), &azservicebus.ReceiverOptions{})
	if err != nil {
		return fmt.Errorf("AzureServiceBus: create receiver (queue=%s)", queue)
	}
	c.receivers.Add(1)
	go func() {
		defer c.receivers.Done()
		for c.ctx.Err() == nil {
			messages, err := receiver.ReceiveMessages(c.ctx, 1, &azservicebus.ReceiveMessagesOptions{})
			if err != nil {
				const backoffTime = time.Minute
				if !errors.Is(err, context.Canceled) {
					log.Ctx(c.ctx).Err(err).Msgf("AzureServiceBus: receive message failed, backing off for %s (queue: %s)", backoffTime, queue)
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
				log.Ctx(c.ctx).Warn().Err(err).Msgf("AzureServiceBus: message handler failed (queue: %s), message will be sent to DLQ", queue)
				if err := receiver.AbandonMessage(c.ctx, azMessage, &azservicebus.AbandonMessageOptions{}); err != nil {
					log.Ctx(c.ctx).Err(err).Msgf("AzureServiceBus: abandon message (for redelivery) failed (queue: %s)", queue)
				}
			} else {
				if err := receiver.CompleteMessage(c.ctx, azMessage, &azservicebus.CompleteMessageOptions{}); err != nil {
					log.Ctx(c.ctx).Err(err).Msgf("AzureServiceBus: complete message failed (queue: %s)", queue)
				}
			}
		}
	}()
	return nil
}

// SendMessage sends a message to the associated Azure Service Bus senders client. It returns an error if the operation fails.
func (c *AzureServiceBusBroker) SendMessage(ctx context.Context, topic string, message *Message) error {
	c.senderLock.RLock()
	defer c.senderLock.RUnlock()
	sender, ok := c.senders[c.topicName(topic)]
	if !ok {
		return fmt.Errorf("AzureServiceBus: sender not found (topic=%s)", topic)
	}
	serviceBusMsg := &azservicebus.Message{
		Body:          message.Body,
		ContentType:   &message.ContentType,
		CorrelationID: message.CorrelationID,
	}
	return sender.SendMessage(ctx, serviceBusMsg, nil)
}

func (c *AzureServiceBusBroker) topicName(topic string) string {
	return c.topicPrefix + topic
}
