//go:generate mockgen -destination=./channels_mock_test.go -package=subscriptions -source=channels.go
package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"io"
	"net/http"
)

// ChannelFactory defines an interface for creating Subscription Notification Channels (e.g. rest-hook).
// A notification channel is the transport that is used to deliver a Subscription Notification to a subscriber.
type ChannelFactory interface {
	Create(ctx context.Context, subscriber fhir.Identifier) (Channel, error)
}

var _ ChannelFactory = CsdChannelFactory{}

// CsdChannelFactory is a ChannelFactory that creates subscription notification channels based on the CSD directory.
// In other words, it looks up the subscriber endpoint in an external registry, the CSD.
// This is typically for out-of-band server-managed FHIR subscriptions.
type CsdChannelFactory struct {
	Directory csd.Directory
	// ChannelHttpClient is the HTTP client used to deliver the notification to the subscriber.
	ChannelHttpClient *http.Client
}

func (c CsdChannelFactory) Create(ctx context.Context, subscriber fhir.Identifier) (Channel, error) {
	endpoint, err := c.Directory.LookupEndpoint(ctx, subscriber, "fhir-notify")
	if err != nil {
		return nil, fmt.Errorf("lookup notification endpoint in CSD: %w", err)
	}
	if len(endpoint) == 0 {
		return nil, fmt.Errorf("no notification endpoint found in CSD for subscriber: %s", coolfhir.ToString(subscriber))
	}
	return &RestHookChannel{
		Endpoint: endpoint[0].Address,
		Client:   c.ChannelHttpClient,
	}, nil
}

// ReceiverFailure is returned when a FHIR subscription could not be delivered to the receiver,
// because the receiver is unreachable or didn't return a response indicating successful delivery.
var ReceiverFailure = errors.New("FHIR subscription could not be delivered to receiver")

type Channel interface {
	Notify(ctx context.Context, notification coolfhir.SubscriptionNotification) error
}

var _ Channel = &RestHookChannel{}

type RestHookChannel struct {
	Endpoint string
	Client   fhirclient.HttpRequestDoer
}

func (r RestHookChannel) Notify(ctx context.Context, notification coolfhir.SubscriptionNotification) error {
	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, io.NopCloser(bytes.NewReader(notificationJSON)))
	if err != nil {
		return err
	}
	httpRequest.Header.Add("Content-Type", fhirclient.FhirJsonMediaType)
	httpResponse, err := r.Client.Do(httpRequest)
	if err != nil {
		return errors.Join(ReceiverFailure, err)
	}
	// Be a good client and read the response, even if we don't actually do anything with it.
	_, _ = io.ReadAll(io.LimitReader(httpResponse.Body, 1024))
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return errors.Join(ReceiverFailure, fmt.Errorf("non-OK HTTP response status: %v", httpResponse.Status))
	}
	return nil
}