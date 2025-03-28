package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

var _ Broker = &HTTPBroker{}

type HTTPBrokerConfig struct {
	Endpoint string `koanf:"endpoint"`
	// TopicFilter is a list of topics that should be sent over HTTP. If empty, all topics are sent.
	TopicFilter []string `koanf:"topicfilter"`
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
	topicFilter      []string
}

func (h HTTPBroker) ReceiveFromQueue(queue Entity, handler func(context.Context, Message) error) error {
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.ReceiveFromQueue(queue, handler)
}

func (h HTTPBroker) Close(ctx context.Context) error {
	if h.underlyingBroker == nil {
		return nil
	}
	return h.underlyingBroker.Close(ctx)
}

func (h HTTPBroker) SendMessage(ctx context.Context, topic Entity, message *Message) error {
	if len(h.topicFilter) != 0 && !slices.Contains(h.topicFilter, topic.Name) {
		return nil
	}
	var errs []error
	if err := h.doSend(ctx, topic, message); err != nil {
		errs = append(errs, fmt.Errorf("failed to send message over HTTP: %w", err))
	}
	if h.underlyingBroker != nil {
		if err := h.underlyingBroker.SendMessage(ctx, topic, message); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (h HTTPBroker) doSend(ctx context.Context, topic Entity, message *Message) error {
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
	httpRequestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(httpRequestCtx, "POST", endpoint.JoinPath(topic.Name).String(), strings.NewReader(string(jsonValue)))
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
