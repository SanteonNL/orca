package pubsub

import (
	"context"
	"fmt"
)

var DefaultSubscribers = Subscribers{
	FhirSubscriptionNotify: func(ctx context.Context, _ any) error {
		return fmt.Errorf("pub/sub FHIR notification subscriber not set")
	},
}

// Subscribers defines a very simple interface for an in-process pub/sub mechanism. Subscribers have to set their function handler on the global DefaultSubscribers variable.
type Subscribers struct {
	// FhirSubscriptionNotify is called when a FHIR subscriber is notified.
	FhirSubscriptionNotify func(ctx context.Context, resource any) error
}
