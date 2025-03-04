package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"sync"
)

// AzureServiceBusConfig holds the configuration for connecting to and interacting with a AzureServiceBus instance.
type AzureServiceBusConfig struct {
	Hostname         string `koanf:"hostname"`
	ConnectionString string `koanf:"connectionstring" description:"This is the connection string for connecting to AzureServiceBus."`
}

func newAzureServiceBusBroker(conf AzureServiceBusConfig, topics []string) (*AzureServiceBusBroker, error) {
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
	return &AzureServiceBusBroker{
		client:  client,
		senders: senders,
	}, nil
}

// AzureServiceBusBroker is an implementation of the MessageBroker interface for interacting with Azure Service Bus.
// It wraps an azservicebus.Sender for sending messages to a specific Service Bus topic.
// This struct provides methods to send messages and close the underlying Service Bus connection.
type AzureServiceBusBroker struct {
	senders    map[string]*azservicebus.Sender
	senderLock sync.RWMutex
	client     *azservicebus.Client
}

// Close releases the underlying resources associated with the AzureServiceBusBroker instance.
func (c *AzureServiceBusBroker) Close(ctx context.Context) error {
	c.senderLock.Lock()
	defer c.senderLock.Unlock()
	// Collect all c lose() errors from senders, receivers and client, then return them as a whole.
	var errs []error
	for topic, sender := range c.senders {
		if err := sender.Close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to close sender (topic=%s): %w", topic, err))
		}
		delete(c.senders, topic)
	}
	if err := c.client.Close(ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to close client: %w", err))
	}
	if len(errs) > 0 {
		return errors.Join(append([]error{
			errors.New("azure service bus: close() failures")},
			errs...,
		)...)
	}
	return nil
}

// SendMessage sends a message to the associated Azure Service Bus senders client. It returns an error if the operation fails.
func (c *AzureServiceBusBroker) SendMessage(ctx context.Context, topic string, message *Message) error {
	c.senderLock.RLock()
	defer c.senderLock.RUnlock()
	sender, ok := c.senders[topic]
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
