//go:generate mockgen -destination=./servicebus_client_mock.go -package=ehr -source=servicebus_client.go
package ehr

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"strings"
)

// ServiceBusConfig holds the configuration for connecting to and interacting with a ServiceBus instance.
type ServiceBusConfig struct {
	Enabled          bool   `koanf:"enabled" default:"false" description:"This enables the ServiceBus client."`
	DebugOnly        bool   `koanf:"debug" default:"false" description:"This enables debug mode for ServiceBus, writing the messages to a file in the OS TempDir instead of sending them to ServiceBus."`
	Topic            string `koanf:"topic"`
	PingOnStartup    bool   `koanf:"ping" default:"false" description:"This enables pinging the ServiceBus client on startup."`
	Hostname         string `koanf:"hostname"`
	ConnectionString string `koanf:"connectionstring" description:"This is the connection string for connecting to ServiceBus."`
}

// ServiceBusClient defines an interface for submitting messages to a Service Bus system.
type ServiceBusClient interface {
	SubmitMessage(ctx context.Context, key string, value string) error
}

// ServiceBusClientImpl provides an implementation for interacting with Azure Service Bus using the provided configuration.
type ServiceBusClientImpl struct {
	config ServiceBusConfig
}

// DebugClient is an implementation of ServiceBusClient that simulates message submission by writing data to the OS TempDir.
type DebugClient struct {
}

// NoopClient is a no-operation implementation of the ServiceBusClient interface, used when ServiceBus is disabled or unused.
type NoopClient struct {
}

// NewClient initializes and returns a ServiceBusClient based on the provided ServiceBusConfig.
// Returns an error if configuration is invalid or initialization fails.
func NewClient(config ServiceBusConfig) (ServiceBusClient, error) {
	ctx := context.Background()
	if config.Enabled {
		if config.DebugOnly {
			log.Info().Ctx(ctx).Msg("Debug mode enabled, writing messages to files in OS temp dir")
			return &DebugClient{}, nil
		}
		kafkaClient := newServiceBusClient(config)
		if config.PingOnStartup {
			log.Info().Ctx(ctx).Msg("Ping enabled, starting ping")
			err := kafkaClient.SubmitMessage(context.Background(), "ping", "pong")
			if err != nil {
				return nil, err
			}
		}
		return kafkaClient, nil
	}
	return &NoopClient{}, nil
}

// newServiceBusClient initializes a new ServiceBusClient instance based on the provided ServiceBusConfig.
var newServiceBusClient = func(config ServiceBusConfig) ServiceBusClient {
	return &ServiceBusClientImpl{
		config: config,
	}
}

// newAzureServiceBusClient initializes a new ServiceBusClientWrapper using the provided ServiceBusConfig.
var newAzureServiceBusClient = func(conf ServiceBusConfig) (ServiceBusClientWrapper, error) {
	return NewServiceBusClientWrapper(conf)
}

// Connect establishes a connection to Azure Service Bus and returns a ServiceBusClientWrapper or an error if it fails.
func (k *ServiceBusClientImpl) Connect() (sender ServiceBusClientWrapper, err error) {
	return newAzureServiceBusClient(k.config)
}

// SubmitMessage sends a message with a specified key and value to the configured Azure Service Bus topic.
func (k *ServiceBusClientImpl) SubmitMessage(ctx context.Context, key string, value string) error {
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitting key %s", key)
	sender, err := k.Connect()
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msgf("Connect failed with %s", err.Error())
		return err
	}
	defer sender.Close(ctx)
	message := &azservicebus.Message{
		Body:          []byte(value),
		CorrelationID: &key,
	}
	err = sender.SendMessage(ctx, message)
	if err != nil {
		return err
	}
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitted key %s", key)
	return nil
}

// SubmitMessage writes the provided value to a temporary JSON file named based on the sanitized key parameter.
// The method logs the operation and returns an error if the file writing fails.
func (k *DebugClient) SubmitMessage(ctx context.Context, key string, value string) error {
	name := path.Join(os.TempDir(), strings.ReplaceAll(key, ":", "_")+".json")
	log.Debug().Ctx(ctx).Msgf("DebugClient, write to file: %s", name)
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		log.Warn().Ctx(ctx).Msgf("DebugClient, failed to write to file: %s, err: %s", name, err.Error())
		return err
	}
	return nil
}

// SubmitMessage sends a message with the given key and value, acting as a no-op in this implementation.
func (k *NoopClient) SubmitMessage(_ context.Context, _ string, _ string) error {
	return nil
}
