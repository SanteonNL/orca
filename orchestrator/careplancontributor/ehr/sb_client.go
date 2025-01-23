//go:generate mockgen -destination=./sb_client_mock.go -package=ehr -source=sb_client.go
package ehr

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
)

type SbClient interface {
	Close(ctx context.Context) error
	SendMessage(ctx context.Context, message *azservicebus.Message) error
}

func NewSbClient(conf ServiceBusConfig) (SbClient, error) {
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
		return &SbClientImpl{sbClient: sender}, nil

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
		return &SbClientImpl{sbClient: sender}, nil
	}
	return nil, errors.New(
		"ServiceBus configuration is missing hostname or connection string")
}

type SbClientImpl struct {
	sbClient *azservicebus.Sender
}

func (c *SbClientImpl) Close(ctx context.Context) error {
	return c.sbClient.Close(ctx)
}
func (c *SbClientImpl) SendMessage(ctx context.Context, message *azservicebus.Message) error {
	return c.sbClient.SendMessage(ctx, message, nil)
}
