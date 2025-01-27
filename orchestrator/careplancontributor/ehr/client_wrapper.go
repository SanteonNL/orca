//go:generate mockgen -destination=./client_wrapper_mock.go -package=ehr -source=client_wrapper.go
package ehr

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

// ServiceBusClientWrapper defines an interface for interacting with Azure Service Bus, including sending messages and closing connections.
type ServiceBusClientWrapper interface {
	Close(ctx context.Context) error
	SendMessage(ctx context.Context, message *azservicebus.Message) error
}

// NewServiceBusClientWrapper initializes and returns a new ServiceBusClientWrapper or an error if configuration is invalid.
func NewServiceBusClientWrapper(conf ServiceBusConfig) (ServiceBusClientWrapper, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	if conf.ConnectionString != "" {
		client, err := azservicebus.NewClientFromConnectionString(conf.ConnectionString, nil)
		if err != nil {
			return nil, err
		}
		sender, err := client.NewSender(conf.Topic, nil)
		if err != nil {
			return nil, err
		}
		return &ServiceBusClientWrapperImpl{sender: sender}, nil

	}
	if conf.Hostname != "" {
		client, err := azservicebus.NewClient(conf.Hostname, cred, nil)
		if err != nil {
			return nil, err
		}
		sender, err := client.NewSender(conf.Topic, nil)
		if err != nil {
			return nil, err
		}
		return &ServiceBusClientWrapperImpl{sender: sender, client: client}, nil
	}
	return nil, errors.New(
		"ServiceBus configuration is missing hostname or connection string")
}

// ServiceBusClientWrapperImpl is an implementation of the ServiceBusClientWrapper interface for interacting with Azure Service Bus.
// It wraps an azservicebus.Sender for sending messages to a specific Service Bus topic.
// This struct provides methods to send messages and close the underlying Service Bus connection.
type ServiceBusClientWrapperImpl struct {
	sender *azservicebus.Sender
	client *azservicebus.Client
}

// Close releases the underlying resources associated with the ServiceBusClientWrapperImpl instance.
func (c *ServiceBusClientWrapperImpl) Close(ctx context.Context) error {
	err := c.sender.Close(ctx)
	if err != nil {
		err = c.client.Close(ctx)
	}
	return err
}

// SendMessage sends a message to the associated Azure Service Bus sender client. It returns an error if the operation fails.
func (c *ServiceBusClientWrapperImpl) SendMessage(ctx context.Context, message *azservicebus.Message) error {
	return c.sender.SendMessage(ctx, message, nil)
}
