package ehr

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/labstack/gommon/log"
	"os"
	"strings"
)

type KafkaConfig struct {
	Enabled          bool   `koanf:"enabled"`
	Topic            string `koanf:"topic"`
	Endpoint         string `koanf:"endpoint"`
	ConnectionString string `koanf:"connectionstring"`
}

type KafkaClient interface {
	SubmitMessage(key string, value string) error
}

type KafkaClientImpl struct {
	topic    string
	producer *kafka.Producer
}

type NoopClientImpl struct {
}

func NewClient(config KafkaConfig) (KafkaClient, error) {

	var kafkaClient KafkaClient
	if config.Enabled {
		endpoint := config.Endpoint
		connectionString := config.ConnectionString
		log.Infof("KafkaClientImpl, connecting to %s, using connectionString %s", endpoint, connectionString)
		producer, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": endpoint,
			"sasl.mechanisms":   "PLAIN",
			"security.protocol": "SASL_PLAINTEXT",
			"sasl.username":     "$ConnectionString",
			"sasl.password":     connectionString,
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

func (k *NoopClientImpl) SubmitMessage(key string, value string) error {
	name := "/tmp/" + strings.ReplaceAll(key, ":", "_") + ".json"
	log.Infof("NoopClientImpl, write to file: %s", name)
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		return err
	}
	return nil
}
