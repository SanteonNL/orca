//go:generate mockgen -destination=./kafka_client_mock.go -package=ehr -source=kafka_client.go
package ehr

import (
	"context"
	"errors"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/oauth"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"net/http"
	"os"
	"path"
	"strings"
)

// KafkaConfig holds the configuratoion settings for connecting to a Kafka broker.
// It includes options to enable Kafka, specify the topic, endpoint, and connection string.
//
// Fields:
//   - Enabled: A boolean indicating whether Kafka is enabled.
//   - Topic: The Kafka topic to which messages will be sent.
//   - Endpoint: The Kafka broker endpoint.
//   - ConnectionString: The connection string used for authentication.
type KafkaConfig struct {
	Enabled       bool           `koanf:"enabled" default:"false" description:"This enables the Kafka client."`
	Demo          bool           `koanf:"demo" default:"false" description:"This enables the Kafka client in demo mode. In demo mode, the Kafka client will send messages to the endpoint over http instead of Kafka."`
	DebugOnly     bool           `koanf:"debug" default:"false" description:"This enables debug mode for Kafka, writing the messages to a file in the OS TempDir instead of sending them to Kafka."`
	PingOnStartup bool           `koanf:"ping" default:"false" description:"This enables pinging the Kafka broker on startup."`
	Topic         string         `koanf:"topic"`
	Endpoint      string         `koanf:"endpoint"`
	Sasl          SaslConfig     `koanf:"sasl"`
	Security      SecurityConfig `koanf:"security"`
}

// SaslConfig holds the configuration settings for SASL authentication.
// It includes the mechanism, username, and password required for authentication
type SaslConfig struct {
	// Mechanism contains the SASL mechanism to use for authentication (e.g., PLAIN).
	Mechanism string `koanf:"mechanism" default:"PLAIN"`
	Username  string `koanf:"username" default:"$ConnectionString"`
	Password  string `koanf:"password"`
}
type SecurityConfig struct {
	Protocol string `koanf:"protocol" default:"SASL_PLAINTEXT"`
}

// KafkaClient is an interface that defines the method for submitting messages.
// Implementations of this interface should provide the logic for submitting
// messages to a Kafka topic or an alternative storage.
//
// Methods:
//   - SubmitMessage: Submits a message with the given key and value.
//
// Parameters:
//   - ctx: The context for the message submission, used for cancellation and timeouts.
//   - key: The key of the message to be submitted.
//   - value: The value of the message to be submitted.
//
// Returns:
//   - error: An error if the message could not be submitted.
type KafkaClient interface {
	SubmitMessage(ctx context.Context, key string, value string) error
	PingConnection(ctx context.Context) error
}

// KafkaClientImpl is an implementation of the KafkaClient interface.
// It holds the Kafka topic and producer used to submit messages.
type KafkaClientImpl struct {
	config KafkaConfig
}

type DebugClient struct {
}

type DemoClient struct {
	messageEndpoint string
}

type NoopClient struct {
}

// NewClient creates a new KafkaClient based on the provided KafkaConfig.
// If Kafka is enabled in the configuration, it initializes a Kafka producer
// and sets up a delivery report handler for produced messages. If Kafka is
// not enabled, it returns a DebugClient which writes messages to a file.
//
// Parameters:
//   - config: KafkaConfig containing the configuration for the Kafka client.
//
// Returns:
//   - KafkaClient: An implementation of the KafkaClient interface.
//   - error: An error if the Kafka producer could not be created.
func NewClient(config KafkaConfig) (KafkaClient, error) {
	if config.Enabled {
		if config.Demo {
			if globals.StrictMode {
				return nil, errors.New("demo mode is not allowed in strict mode")
			}
			log.Info().Msgf("Demo mode enabled, sending messages over http to endpoint: %s", config.Endpoint)

			return &DemoClient{
				messageEndpoint: config.Endpoint,
			}, nil
		}
		if config.DebugOnly {
			ctx := context.Background()
			log.Info().Ctx(ctx).Msg("Debug mode enabled, writing messages to files in OS temp dir")
			return &DebugClient{}, nil
		}
		kafkaClient := newKafkaClient(config)
		if config.PingOnStartup {
			err := kafkaClient.PingConnection(context.Background())
			if err != nil {
				log.Error().Err(err).Msgf("PingOnStartup failed with %s", err.Error())
				return nil, err
			}
		}
		return kafkaClient, nil
	}
	return &NoopClient{}, nil
}

var newKafkaClient = func(config KafkaConfig) KafkaClient {
	return &KafkaClientImpl{
		config: config,
	}
}

// CreateSaslClient initializes and returns a Kafka client configured with SASL authentication and optional TLS support.
func (k *KafkaClientImpl) CreateSaslClient(ctx context.Context, useTls bool) (KgoClient, error) {
	endpoint := k.config.Endpoint
	seeds := []string{endpoint}
	mechanism := k.config.Sasl.Mechanism
	switch mechanism {
	case "OAUTHBEARER":
		bearerToken, err := getAccessToken(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		opts := []kgo.Opt{
			kgo.SeedBrokers(seeds...),
			// SASL Options
			kgo.SASL(oauth.Auth{
				Token: bearerToken.Token,
			}.AsMechanism()),
			// Needed for Microsoft Event Hubs
			kgo.ProducerBatchCompression(kgo.NoCompression()),
			kgo.DefaultProduceTopic(k.config.Topic),
		}
		if useTls {
			opts = append(opts, kgo.DialTLS())
		}
		client, err := newKgoClient(opts)
		if err != nil {
			return nil, err
		}
		log.Info().Msgf("PingOnStartup is set to %t", k.config.PingOnStartup)

		return client, nil
	case "PLAIN":
		username := k.config.Sasl.Username
		password := k.config.Sasl.Password
		opts := []kgo.Opt{
			kgo.SeedBrokers(seeds...),
			// SASL Options
			kgo.SASL(plain.Auth{
				User: username,
				Pass: password,
			}.AsMechanism()),
			// Needed for Microsoft Event Hubs
			kgo.ProducerBatchCompression(kgo.NoCompression()),
			kgo.DefaultProduceTopic(k.config.Topic),
		}
		if useTls {
			opts = append(opts, kgo.DialTLS())
		}
		client, err := newKgoClient(opts)
		if err != nil {
			return nil, err
		}

		return client, nil
	default:
		err := errors.New("Unsupported mechanism: " + mechanism)
		return nil, err
	}
}

var newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
	return NewKgoClientWithOpts(opts)
}

// Connect establishes a connection to Kafka using the protocol specified in the configuration.
// It supports SASL_SSL and SASL_PLAINTEXT protocols for authentication.
// Returns a Kafka client or an error if the connection fails or an unsupported protocol is specified.
func (k *KafkaClientImpl) Connect(ctx context.Context) (client KgoClient, err error) {
	switch k.config.Security.Protocol {
	case "SASL_SSL":
		return k.CreateSaslClient(ctx, true)
	case "SASL_PLAINTEXT":
		return k.CreateSaslClient(ctx, false)
	default:
		err := errors.New("Unsupported protocol: " + k.config.Security.Protocol)
		return nil, err
	}
}

// getAccessToken retrieves an access token for a given Azure endpoint using an OAuth client.
// It initializes the OAuth client, fetches Azure credentials, and acquires a bearer token.
// Parameters:
// - ctx: The context for managing request deadlines and cancellation.
// - endpoint: The Azure resource endpoint for which the access token is required.
// Returns:
// - *azcore.AccessToken: Retrieved access token containing the token string and expiry.
// - error: An error if token retrieval or any intermediate operation fails.
var getAccessToken = func(ctx context.Context, endpoint string) (*azcore.AccessToken, error) {
	oauthClient, err := newAzureOauthClient()
	if err != nil {
		return nil, err
	}
	principalToken, err := oauthClient.GetAzureCredential()
	if err != nil {
		return nil, err
	}
	bearerToken, err := oauthClient.GetBearerToken(ctx, principalToken, endpoint)
	if err != nil {
		return nil, err
	}
	return bearerToken, nil
}

// PingConnection attempts to establish and verify a connection to Kafka by invoking Ping on the Kafka client.
// It logs a success message if Kafka is reachable or an error message if the ping fails and returns the error.
// Parameters:
//   - ctx: Context used to carry deadlines, cancellation signals, and other request-scoped values.
//
// Returns:
//   - error: An error if the connection or ping operation fails.
func (k *KafkaClientImpl) PingConnection(ctx context.Context) error {
	client, err := k.Connect(ctx)
	if err != nil {
		log.Error().Err(err).Msgf("Connect failed with %s", err.Error())
	}
	err = client.Ping(ctx)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msgf("Failed to ping Kafka, message: %s", err.Error())
		return err
	} else {
		log.Info().Ctx(ctx).Msg("Pinged Kafka successfully on startup")
	}
	return nil
}

// SubmitMessage submits a message to the Kafka topic associated with the KafkaClientImpl.
// It produces a message with the given key and value to the Kafka producer.
//
// Parameters:
//   - key: The key of the message to be submitted.
//   - value: The value of the message to be submitted.
//
// Returns:
//   - error: An error if the message could not be produced.
func (k *KafkaClientImpl) SubmitMessage(ctx context.Context, key string, value string) error {
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitting key %s", key)
	client, err := k.Connect(ctx)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msgf("Connect failed with %s", err.Error())
		return err
	}
	record := kgo.KeyStringRecord(key, value)
	record.Topic = k.config.Topic
	sync := client.ProduceSync(ctx, record)
	var lastErr error
	for _, s := range sync {
		if s.Err != nil {
			log.Error().Ctx(ctx).Err(s.Err).Msgf("Error during submission %s, with topic %s", s.Err.Error(), record.Topic)
			lastErr = s.Err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	// Make sure all messages are flushed before returning
	err = client.Flush(ctx)
	if err != nil {
		log.Error().Ctx(ctx).Err(err).Msgf("kafka flush failed %s", err.Error())
		return err
	}
	log.Debug().Ctx(ctx).Msgf("SubmitMessage, submitted key %s", key)
	return nil
}

// SubmitMessage writes a message to a file in the /tmp directory.
// The filename is derived from the key by replacing colons with underscores
// and appending the .json extension.
//
// Parameters:
//   - key: The key of the message, used to generate the filename.
//   - value: The value of the message, written to the file.
//
// Returns:
//   - error: An error if the file could not be written.
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

func (k *DebugClient) PingConnection(ctx context.Context) error {
	log.Debug().Ctx(ctx).Msgf("DebugClient: pong")
	return nil
}

// SubmitMessage submits a message to the configured endpoint over http.
//
// Parameters:
//   - key: The key of the message to be submitted.
//   - value: The value of the message to be submitted.
//
// Returns:
//   - error: An error if the message could not be produced.
func (k *DemoClient) SubmitMessage(ctx context.Context, key string, value string) error {
	jsonValue := strings.ReplaceAll(value, " ", "")
	jsonValue = strings.ReplaceAll(jsonValue, "\n", "")
	jsonValue = strings.ReplaceAll(jsonValue, "\t", "")
	log.Debug().Ctx(ctx).Msgf("DemoClient, submitting message %s - %s", key, jsonValue)

	req, err := http.NewRequestWithContext(ctx, "POST", k.messageEndpoint, strings.NewReader(jsonValue))
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

	log.Debug().Ctx(ctx).Msgf("DemoClient, successfully sent message to endpoint")
	return nil
}

func (k *DemoClient) PingConnection(ctx context.Context) error {
	log.Debug().Ctx(ctx).Msgf("DemoClient: pong")
	return nil
}

func (k *NoopClient) PingConnection(ctx context.Context) error {
	return nil
}

func (k *NoopClient) SubmitMessage(ctx context.Context, key string, value string) error {
	return nil
}
