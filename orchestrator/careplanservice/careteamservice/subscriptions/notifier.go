package subscriptions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"io"
	"net/http"
)

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
