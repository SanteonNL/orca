package subscriptions

import (
	"context"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/csd"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
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
	Directory        csd.Directory
	DirectoryService string
	// ChannelHttpClient is the HTTP client used to deliver the notification to the subscriber.
	ChannelHttpClient *http.Client
}

func (c CsdChannelFactory) Create(ctx context.Context, subscriber fhir.Identifier) (Channel, error) {
	endpoint, err := c.Directory.LookupEndpoint(ctx, subscriber, c.DirectoryService, "fhir-notify")
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
