package ehr

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/labstack/gommon/log"
	"os"
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
	Enabled  bool           `koanf:"enabled" default:"false"`
	Topic    string         `koanf:"topic"`
	Endpoint string         `koanf:"endpoint"`
	Sasl     SaslConfig     `koanf:"sasl"`
	Security SecurityConfig `koanf:"security"`
}

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
//   - key: The key of the message to be submitted.
//   - value: The value of the message to be submitted.
//
// Returns:
//   - error: An error if the message could not be submitted.
type KafkaClient interface {
	SubmitMessage(key string, value string) error
}

// KafkaClientImpl is an implementation of the KafkaClient interface.
// It holds the Kafka topic and producer used to submit messages.
type KafkaClientImpl struct {
	topic    string
	producer *kafka.Producer
}

type NoopClientImpl struct {
}

// NewClient creates a new KafkaClient based on the provided KafkaConfig.
// If Kafka is enabled in the configuration, it initializes a Kafka producer
// and sets up a delivery report handler for produced messages. If Kafka is
// not enabled, it returns a NoopClientImpl which writes messages to a file.
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
		endpoint := config.Endpoint
		log.Infof("KafkaClientImpl, connecting to %s, Mechanism: %s, protocol: %s, username: %s, password: %s",
			endpoint,
			config.Sasl.Mechanism,
			config.Security.Protocol,
			config.Sasl.Username,
			config.Sasl.Password)
		producer, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": endpoint,
			"sasl.mechanisms":   config.Sasl.Mechanism,
			"sasl.username":     config.Sasl.Username,
			"sasl.password":     config.Sasl.Password,
			"security.protocol": config.Security.Protocol,
		})
		if err != nil {
			return nil, err
		}

		// Delivery report handler for produced messages
		go func() {
			log.Infof("Kafka func started")
			for e := range producer.Events() {
				log.Infof("Kafka event received: %v", e.String())
				switch ev := e.(type) {
				case *kafka.Message:
					if ev.TopicPartition.Error != nil {
						log.Infof("Kafka delivery failed: %v\n", ev.TopicPartition)
					} else {
						log.Errorf("Kafka delivered message to %v\n", ev.TopicPartition)
					}
				}
			}
			log.Infof("Kafka func ended")
		}()

		kafkaClient = &KafkaClientImpl{
			topic:    config.Topic,
			producer: producer,
		}
	} else {
		kafkaClient = &NoopClientImpl{}
	}
	return kafkaClient, nil
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
func (k *KafkaClientImpl) SubmitMessage(key string, value string) error {
	log.Infof("SubmitMessage, submitting key %s", key)
	err := k.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &k.topic,
			Partition: kafka.PartitionAny,
		},
		Key:   []byte(key),
		Value: []byte(value),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	k.producer.Flush(10 * 1000)
	log.Infof("SubmitMessage, submitted key %s", key)
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
func (k *NoopClientImpl) SubmitMessage(key string, value string) error {
	name := "/tmp/" + strings.ReplaceAll(key, ":", "_") + ".json"
	log.Infof("NoopClientImpl, write to file: %s", name)
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		return err
	}
	return nil
}
