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
	if config.AzureServiceBus.Enabled() {
		broker, err = newAzureServiceBusBroker(config.AzureServiceBus, topics, config.TopicPrefix)
		if err != nil {
			return nil, fmt.Errorf("azure service bus: %w", err)
		}
	} else {
		// If no configuration is provided, default to an in-memory broker
		log.Warn().Msg("No messaging configuration provided, defaulting to in-memory broker. " +
			"This is unsuitable for production use, since failed messages can't be retried.")
		broker = NewMemoryBroker()
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
	// TopicPrefix is the prefix to use for all topics, which allows for multi-tenant use of the underlying message broker infrastructure.
	TopicPrefix string `koanf:"topicprefix"`
}

func (c Config) Validate(strictMode bool) error {
	if strictMode && c.HTTP.Endpoint != "" {
		return errors.New("http endpoint is not allowed in strict mode")
	}
	// TODO: enable when Azure ServiceBus is required for robust operation
	//if !c.AzureServiceBus.Enabled() && strictMode {
	//	return errors.New("production-grade messaging configuration (Azure ServiceBus) is required in strict mode")
	//}
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
	// Receive subscribes to a queue and calls the handler function for each message received.
	// The handler function should return an error if the message processing fails, which will cause the message to be retried or sent to the DLQ.
	Receive(queue string, handler func(context.Context, Message) error) error
}
