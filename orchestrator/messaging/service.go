//go:generate mockgen -destination=./service_mock.go -package=messaging -source=service.go
package messaging

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog/log"
)

// Entity represents a source of messages, such as a topic or queue.
type Entity struct {
	Name string
	// Prefix indicates whether the entity name should be prefixed with the EntityPrefix from the messaging configuration.
	Prefix bool
}

// FullName returns the full name of the entity, optionally prefixed with the provided prefix.
func (t Entity) FullName(prefix string) string {
	if t.Prefix {
		return prefix + t.Name
	}
	return t.Name
}

func New(config Config, sources []Entity) (Broker, error) {
	var broker Broker
	var err error
	log.Info().Msgf("Messaging: CONFIG %+v", config)
	if config.AzureServiceBus.Enabled() {
		broker, err = newAzureServiceBusBroker(config.AzureServiceBus, sources, config.EntityPrefix)

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
	if config.DataHub.Endpoint != "" {
		log.Info().Msgf("Messaging: sending messages to DataHub at %s", config.DataHub.Endpoint)
		broker = NewDataHubBroker(config.DataHub, broker)
	}
	return broker, nil
}

// Config holds the configuration for messaging.
type Config struct {
	// AzureServiceBus holds the configuration for messaging using Azure ServiceBus.
	AzureServiceBus AzureServiceBusConfig `koanf:"azureservicebus"`
	HTTP            HTTPBrokerConfig      `koanf:"http"`
	DataHub         DataHubBrokerConfig   `koanf:"datahub"`
	// EntityPrefix is the prefix to use for all topics and queues, which allows for multi-tenant use of the underlying message broker infrastructure.
	EntityPrefix string `koanf:"entityprefix"`
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
	SendMessage(ctx context.Context, topic Entity, message *Message) error
	// ReceiveFromQueue subscribes to a queue and calls the handler function for each message received.
	// The handler function should return an error if the message processing fails, which will cause the message to be retried or sent to the DLQ.
	ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error
}
