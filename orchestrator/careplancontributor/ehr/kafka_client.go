package ehr

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"os"
	"strings"
)

type KafkaConfig struct {
	Enabled bool `koanf:"enabled"`
	// BaseURL is the base URL of the FHIR server to connect to.
	Topic string `koanf:"topic"` // orca-patient-enrollment-events
	// Auth is the authentication configuration for the FHIR server.
	Address []string `koanf:"address"`
}

type KafkaClient interface {
	SubmitMessage(key string, value string) error
}

type KafkaClientImpl struct {
	writer *kafka.Writer
}

type NoopClientImpl struct {
}

func NewClient(config KafkaConfig) KafkaClient {
	var kafkaClient KafkaClient
	if config.Enabled {
		writer := &kafka.Writer{
			Addr:     kafka.TCP(config.Address...),
			Topic:    config.Topic,
			Balancer: &kafka.LeastBytes{},
		}
		kafkaClient = &KafkaClientImpl{
			writer: writer,
		}
	} else {
		kafkaClient = &NoopClientImpl{}
	}
	return kafkaClient
}

func (k *KafkaClientImpl) SubmitMessage(key string, value string) error {
	msg := kafka.Message{
		Key:   []byte(key),
		Value: []byte(value),
	}

	err := k.writer.WriteMessages(context.Background(), msg)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	return nil
}

func (k *NoopClientImpl) SubmitMessage(key string, value string) error {
	name := "/tmp/" + strings.ReplaceAll(key, ":", "_") + ".json"
	println(fmt.Sprintf("NoopClientImpl, write to file: %s", name))
	err := os.WriteFile(name, []byte(value), 0644)
	if err != nil {
		return err
	}
	return nil
}
