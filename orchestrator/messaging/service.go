//go:generate mockgen -destination=./service_mock.go -package=messaging -source=service.go
package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
)

func New(config Config, topics []string) (Broker, error) {
	var broker Broker
	var err error
	if config.AzureServiceBus.ConnectionString != "" || config.AzureServiceBus.Hostname != "" {
		broker, err = newAzureServiceBusBroker(config.AzureServiceBus, topics)
		if err != nil {
			return nil, fmt.Errorf("azure service bus: %w", err)
		}
	}
	if config.HTTP.Endpoint != "" {
		log.Info().Msgf("Messaging: sending messages over HTTP to %s", config.HTTP.Endpoint)
		broker = NewHTTPBroker(config.HTTP, broker)
	}
	return broker, nil
}

// Config holds the configuration for messaging.
type Config struct {
	// AzureServiceBus holds the configuration for messaging using Azure ServiceBus.
	AzureServiceBus AzureServiceBusConfig `koanf:"azureservicebus"`
	HTTP            HTTPBrokerConfig      `koanf:"http"`
}

func (c Config) Validate(strictMode bool) error {
	if strictMode && c.HTTP.Endpoint != "" {
		return errors.New("http endpoint is not allowed in strict mode")
	}
	return nil
}

type Message struct {
	Body          []byte
	ContentType   string
	CorrelationID *string
}

// Broker defines an interface for interacting with a message broker, including sending messages and closing connections.
type Broker interface {
	Close(ctx context.Context) error
	SendMessage(ctx context.Context, topic string, message *Message) error
}
