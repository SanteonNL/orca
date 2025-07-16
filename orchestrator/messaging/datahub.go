package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

var _ Broker = &DataHubBroker{}

type DataHubBrokerConfig struct {
	Endpoint string `koanf:"endpoint"`
}

func NewDataHubBroker(config DataHubBrokerConfig, underlyingBroker Broker) Broker {
	return DataHubBroker{
		underlyingBroker: underlyingBroker,
		endpoint:         config.Endpoint,
	}
}

type DataHubBroker struct {
	underlyingBroker Broker
	endpoint         string
}

func (h DataHubBroker) ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error {
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.ReceiveFromQueue(queue, handler)
}

func (h DataHubBroker) Close(ctx context.Context) error {
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.Close(ctx)
}

func (h DataHubBroker) SendMessage(ctx context.Context, topic Entity, message *Message) error {

	log.Ctx(ctx).Debug().Msgf("SendMessage invoked for topic %s. ", topic.Name)

	var errs []error
	if err := h.doSend(ctx, message); err != nil {
		errs = append(errs, fmt.Errorf("failed to send message over HTTP: %w", err))
	}
	if h.underlyingBroker != nil {
		if err := h.underlyingBroker.SendMessage(ctx, topic, message); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		log.Ctx(ctx).Debug().Msgf("Sent message to topic %s", topic.Name)
	}
	return errors.Join(errs...)
}

func (h DataHubBroker) doSend(ctx context.Context, message *Message) error {

	// unmarshall and marshall the value to remove extra whitespace
	var v interface{}
	err := json.Unmarshal(message.Body, &v)
	if err != nil {
		return err
	}
	jsonValue, err := json.Marshal(v)
	if err != nil {
		return err
	}
	log.Ctx(ctx).Info().Msgf("Sending json message %s", jsonValue)
	endpoint, err := url.Parse(h.endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}
	httpRequestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(httpRequestCtx, http.MethodPost, endpoint.String(), strings.NewReader(string(jsonValue)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", message.ContentType)
	log.Ctx(ctx).Debug().Msgf("Sending message to %s", req.URL.String())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %d with body: %v", resp.StatusCode, resp.Body)
	}
	return nil
}
