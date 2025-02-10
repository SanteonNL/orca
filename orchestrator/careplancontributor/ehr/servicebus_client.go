//go:generate mockgen -destination=./servicebus_client_mock.go -package=ehr -source=servicebus_client.go
package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/rs/zerolog/log"
	"net/http"
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
	DemoEndpoint     string `koanf:"demoendpoint" description:"This is the endpoint for the demo client. If this is set then task notifications will be sent over http to this endpoint"`
}

// ServiceBusClient defines an interface for submitting messages to a Service Bus system.
type ServiceBusClient interface {
	SubmitMessage(ctx context.Context, key string, value string) error
}

// ServiceBusClientImpl provides an implementation for interacting with Azure Service Bus using the provided configuration.
type ServiceBusClientImpl struct {
	config     ServiceBusConfig
	demoClient *DemoClient
}

// DebugClient is an implementation of ServiceBusClient that simulates message submission by writing data to the OS TempDir.
type DebugClient struct {
}

// DemoClient is an implementation of ServiceBusClient that simulates message submission by sending it over HTTP to the supplied endpoint
type DemoClient struct {
	messageEndpoint string
}

// NoopClient is a no-operation implementation of the ServiceBusClient interface, used when ServiceBus is disabled or unused.
type NoopClient struct {
}

// NewClient initializes and returns a ServiceBusClient based on the provided ServiceBusConfig.
// Returns an error if configuration is invalid or initialization fails.
func NewClient(config ServiceBusConfig) (ServiceBusClient, error) {
	ctx := context.Background()
	var demoClient *DemoClient

	if config.Enabled {
		if config.DemoEndpoint != "" {
			if globals.StrictMode {
				return nil, errors.New("servicebus demo endpoint is not allowed in strict mode")
			}
			log.Ctx(ctx).Info().Msgf("Demo mode enabled, sending messages over http to %s", config.DemoEndpoint)
			demoClient = &DemoClient{messageEndpoint: config.DemoEndpoint}
		}
		if config.DebugOnly {
			log.Ctx(ctx).Info().Msg("Debug mode enabled, writing messages to files in OS temp dir")
			return &DebugClient{}, nil
		}
		kafkaClient := newServiceBusClient(config)
		if config.PingOnStartup {
			log.Ctx(ctx).Info().Msg("Ping enabled, starting ping")
			err := kafkaClient.SubmitMessage(context.Background(), "ping", "pong")
			if err != nil {
				return nil, err
			}
		}
		return &ServiceBusClientImpl{config: config, demoClient: demoClient}, nil
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
	log.Ctx(ctx).Debug().Msgf("SubmitMessage, submitting key %s", key)
	// TODO: if statement temporary while servicebus is down, remove ASAP
	if k.demoClient == nil {
		sender, err := k.Connect()
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("Connect failed with %s", err.Error())
			return err
		}
		defer func(sender ServiceBusClientWrapper, ctx context.Context) {
			err := sender.Close(ctx)
			if err != nil {
				log.Ctx(ctx).Error().Err(err).Msgf("Close failed with %s", err.Error())
			}
		}(sender, ctx)
		message := &azservicebus.Message{
			Body:          []byte(value),
			CorrelationID: &key,
		}
		err = sender.SendMessage(ctx, message)
		if err != nil {
			return err
		}
		log.Ctx(ctx).Debug().Msgf("SubmitMessage, submitted key %s", key)
	}

	if k.demoClient != nil {
		err := k.demoClient.SubmitMessage(ctx, key, value)
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msgf("DemoClient submission failed with %s", err.Error())
			return err
		}
	}

	return nil
}

// SubmitMessage writes the provided value to a temporary JSON file named based on the sanitized key parameter.
// The method logs the operation and returns an error if the file writing fails.
func (k *DebugClient) SubmitMessage(ctx context.Context, key string, value string) error {
	name := path.Join(os.TempDir(), strings.ReplaceAll(key, ":", "_")+".json")
	log.Ctx(ctx).Debug().Msgf("DebugClient, write to file: %s", name)
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		log.Ctx(ctx).Warn().Msgf("DebugClient, failed to write to file: %s, err: %s", name, err.Error())
		return err
	}
	return nil
}

// SubmitMessage submits a message to the configured endpoint over http.
func (k *DemoClient) SubmitMessage(ctx context.Context, key string, value string) error {
	// unmarshall and marshall the value to remove extra whitespace
	var v interface{}
	err := json.Unmarshal([]byte(value), &v)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("DemoClient, failed to unmarshal value: %s", value)
		return err
	}

	jsonValue, err := json.Marshal(v)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msgf("DemoClient, failed to marshal value: %s", value)
		return err
	}

	log.Ctx(ctx).Debug().Msgf("DemoClient, submitting message %s - %s", key, jsonValue)
	req, err := http.NewRequestWithContext(ctx, "POST", k.messageEndpoint, strings.NewReader(string(jsonValue)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("DemoClient, received non-OK response: %d", resp.StatusCode)
		return err
	}
	log.Ctx(ctx).Debug().Msgf("DemoClient, successfully sent message to endpoint")
	return nil
}

// SubmitMessage sends a message with the given key and value, acting as a no-op in this implementation.
func (k *NoopClient) SubmitMessage(_ context.Context, _ string, _ string) error {
	return nil
}
