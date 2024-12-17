//go:generate mockgen -destination=./kafka_client_mock.go -package=ehr -source=kafka_client.go
package ehr

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"os"
	"path"
	"strings"
)

// KafkaConfig holds the configuration settings for connecting to a Kafka broker.
// It includes options to enable Kafka, specify the topic, endpoint, and connection string.
//
// Fields:
//   - Enabled: A boolean indicating whether Kafka is enabled.
//   - Topic: The Kafka topic to which messages will be sent.
//   - Endpoint: The Kafka broker endpoint.
//   - ConnectionString: The connection string used for authentication.
type KafkaConfig struct {
	Enabled   bool           `koanf:"enabled" default:"false" description:"This enables the Kafka client."`
	DebugOnly bool           `koanf:"debug" default:"false" description:"This enables debug mode for Kafka, writing the messages to a file in the OS TempDir instead of sending them to Kafka."`
	Topic     string         `koanf:"topic"`
	Endpoint  string         `koanf:"endpoint"`
	Sasl      SaslConfig     `koanf:"sasl"`
	Security  SecurityConfig `koanf:"security"`
}

// SaslConfig holds the configuration settings for SASL authentication.
// It includes the mechanism, username, and password required for authentication.
//
// Fields:
//   - Mechanism: The SASL mechanism to use for authentication (e.g., PLAIN).
//   - Username: The username for SASL authentication.
//   - Password: The password for SASL authentication.
type SaslConfig struct {
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
}

// KafkaClientImpl is an implementation of the KafkaClient interface.
// It holds the Kafka topic and producer used to submit messages.
type KafkaClientImpl struct {
	topic  string
	client *kgo.Client
}

type DebugClient struct {
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
	var kafkaClient KafkaClient
	if config.Enabled {
		if config.DebugOnly {
			ctx := context.Background()
			log.Info().Ctx(ctx).Msg("Debug mode enabled, writing messages to files in OS temp dir")
			return &DebugClient{}, nil
		}
		switch config.Security.Protocol {
		case "SASL_PLAINTEXT":
			return CreateSaslClient(config, kafkaClient)
		default:
			err := errors.New("Unsupported protocol: " + config.Security.Protocol)
			return nil, err

		}

	}
	return &NoopClient{}, nil
}

// CreateSaslClient creates a new Kafka client with SASL authentication based on the provided KafkaConfig.
// It supports the PLAIN mechanism for SASL authentication.
//
// Parameters:
//   - config: KafkaConfig containing the configuration for the Kafka client.
//   - kafkaClient: KafkaClient interface to be initialized.
//
// Returns:
//   - KafkaClient: An implementation of the KafkaClient interface.
//   - error: An error if the Kafka client could not be created or if the mechanism is unsupported.
func CreateSaslClient(config KafkaConfig, kafkaClient KafkaClient) (KafkaClient, error) {
	endpoint := config.Endpoint
	seeds := []string{endpoint}
	mechanism := config.Sasl.Mechanism
	switch mechanism {
	case "PLAIN":
		username := config.Sasl.Username
		password := config.Sasl.Password
		opts := []kgo.Opt{
			kgo.SeedBrokers(seeds...),
			// SASL Options
			kgo.SASL(plain.Auth{
				User: username,
				Pass: password,
			}.AsMechanism()),
			// Needed for Microsoft Event Hubs
			kgo.ProducerBatchCompression(kgo.NoCompression()),
		}
		client, err := kgo.NewClient(opts...)
		if err != nil {
			return nil, err
		}

		kafkaClient = &KafkaClientImpl{
			topic:  config.Topic,
			client: client,
		}
		return kafkaClient, nil
	default:
		err := errors.New("Unsupported mechanism: " + mechanism)
		return nil, err
	}
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
	record := kgo.KeyStringRecord(key, value)
	record.Topic = k.topic
	sync := k.client.ProduceSync(ctx, record)
	for _, s := range sync {
		if s.Err != nil {
			log.Error().Ctx(ctx).Msgf("Error during submission %s", s.Err.Error())
			return s.Err
		}
	}
	// Make sure all messages are flushed before returning
	err := k.client.Flush(ctx)
	if err != nil {
		log.Error().Ctx(ctx).Msgf("kafka flush failed %s", err.Error())
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
	name := path.Join(os.TempDir(), strings.ReplaceAll(key, ":", "_") + ".json")
	log.Debug().Ctx(ctx).Msgf("DebugClient, write to file: %s", name)
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		log.Warn().Ctx(ctx).Msgf("DebugClient, failed to write to file: %s, err: %s", name, err.Error())
		return err
	}
	return nil
}

func (k *NoopClient) SubmitMessage(ctx context.Context, key string, value string) error {
	return nil
}
