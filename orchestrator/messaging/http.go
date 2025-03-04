package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
	"strings"
)

var _ Broker = &HTTPBroker{}

type HTTPBrokerConfig struct {
	Endpoint string `koanf:"endpoint"`
}

func NewHTTPBroker(config HTTPBrokerConfig, underlyingBroker Broker) Broker {
	return HTTPBroker{
		underlyingBroker: underlyingBroker,
		endpoint:         config.Endpoint,
	}
}

type HTTPBroker struct {
	underlyingBroker Broker
	endpoint         string
}

func (h HTTPBroker) Close(ctx context.Context) error {
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.Close(ctx)
}

func (h HTTPBroker) SendMessage(ctx context.Context, topic string, message *Message) error {
	go func(msg *Message) {
		if err := h.doSend(ctx, topic, msg); err != nil {
			log.Ctx(ctx).Err(err).Msg("Messaging: failed to send message to HTTP endpoint")
		} else {
			log.Ctx(ctx).Debug().Msgf("Messaging: successfully sent message to HTTP endpoint")
		}
	}(message)
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.SendMessage(ctx, topic, message)
}

func (h HTTPBroker) doSend(ctx context.Context, topic string, message *Message) error {
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

	endpoint, err := url.Parse(h.endpoint)
	if err != nil {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint.JoinPath(topic).String(), strings.NewReader(string(jsonValue)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", message.ContentType)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %d", resp.StatusCode)
	}
	return nil
}
