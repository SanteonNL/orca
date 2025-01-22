//go:generate mockgen -destination=./servicebus_client_mock.go -package=ehr -source=servicebus_client.go
package ehr

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"strings"
)

// ServiceBusConfig represents configurations for connecting to ServiceBus, including enabling, debugging, and hostname.
type ServiceBusConfig struct {
	Enabled          bool   `koanf:"enabled" default:"false" description:"This enables the ServiceBus client."`
	DebugOnly        bool   `koanf:"debug" default:"false" description:"This enables debug mode for ServiceBus, writing the messages to a file in the OS TempDir instead of sending them to ServiceBus."`
	Topic            string `koanf:"topic"`
	PingOnStartup    bool   `koanf:"ping" default:"false" description:"This enables pinging the ServiceBus client on startup."`
	Hostname         string `koanf:"hostname"`
	ConnectionString string `koanf:"connectionstring" description:"This is the connection string for connecting to ServiceBus."`
}

// ServiceBusClient defines an interface for submitting messages to a service bus.
type ServiceBusClient interface {
	SubmitMessage(ctx context.Context, key string, value string) error
}

// ServiceBusClientImpl is an implementation of the ServiceBusClient interface for sending messages to Azure Service Bus.
type ServiceBusClientImpl struct {
	config ServiceBusConfig
}

// DebugClient is a ServiceBusClient implementation used for debugging purposes by writing messages to temporary files.
type DebugClient struct {
}

// NoopClient is a no-operation client implementing the ServiceBusClient interface for disabled configurations.
type NoopClient struct {
}

// NewClient initializes a ServiceBus client based on the provided configuration.
// If ServiceBus is disabled, it returns a NoopClient.
// If debug mode is enabled, it returns a DebugClient for writing messages to files.
// Otherwise, it returns a ServiceBus client for interacting with the configured messaging system.
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

// newServiceBusClient is a factory function that initializes a new instance of ServiceBusClient with the given configuration.
var newServiceBusClient = func(config ServiceBusConfig) ServiceBusClient {
	return &ServiceBusClientImpl{
		config: config,
	}
}

// newAzureServiceBusClient creates a new Azure Service Bus client using the provided configuration and default credentials.
var newAzureServiceBusClient = func(conf ServiceBusConfig) (*azservicebus.Client, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	if conf.ConnectionString != "" {
		return azservicebus.NewClientFromConnectionString(conf.ConnectionString, nil)
	}
	if conf.Hostname != "" {
		return azservicebus.NewClient(conf.Hostname, cred, nil)
	}
	return nil, errors.New(
		"ServiceBus configuration is missing hostname or connection string")
}

// Connect establishes a connection to an Azure Service Bus topic and returns a message sender or an error if connection fails.
func (k *ServiceBusClientImpl) Connect(ctx context.Context) (sender *azservicebus.Sender, err error) {
	client, err := newAzureServiceBusClient(k.config)
	if err != nil {
		return nil, err
	}
	return client.NewSender(k.config.Topic, nil)

}

// SubmitMessage submits a message to Azure Service Bus with the given key and value.
// It establishes a connection, sends the message, and handles any errors during the process.
func (k *ServiceBusClientImpl) SubmitMessage(ctx context.Context, key string, value string) error {
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitting key %s", key)
	sender, err := k.Connect(ctx)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msgf("Connect failed with %s", err.Error())
		return err
	}
	defer sender.Close(ctx)
	message := &azservicebus.Message{
		Body:          []byte(value),
		CorrelationID: &key,
	}
	err = sender.SendMessage(ctx, message, nil)
	if err != nil {
		return err
	}
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitted key %s", key)
	return nil
}

// SubmitMessage writes the provided key-value pair to a file in the OS's temporary directory as a JSON file.
// It replaces colons in the key with underscores to form the filename and returns an error if the operation fails.
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

// PingConnection checks the connectivity of the DebugClient and logs a debug message with "pong" for verification.
func (k *DebugClient) PingConnection(ctx context.Context) error {
	log.Debug().Ctx(ctx).Msgf("DebugClient: pong")
	return nil
}

// PingConnection checks the connectivity of the NoopClient. It always returns nil to simulate a successful ping.
func (k *NoopClient) PingConnection(ctx context.Context) error {
	return nil
}

// SubmitMessage submits a key-value pair as a message using the NoopClient.
// Returns an error if the operation fails.
// ctx is the context for managing request deadlines or cancellation.
// key is the identifier associated with the message.
// value is the content of the message to be submitted.
func (k *NoopClient) SubmitMessage(ctx context.Context, key string, value string) error {
	return nil
}
