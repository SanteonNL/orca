package subscriptions

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestRestHookChannel_Notify(t *testing.T) {
	baseURL, _ := url.Parse("http://example.com/fhir")
	timestamp := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	subscription := fhir.Reference{Reference: to.Ptr("Subscription/123")}
	focus := fhir.Reference{Reference: to.Ptr("Patient/123")}
	notification := coolfhir.CreateSubscriptionNotification(baseURL, timestamp, subscription, 1, focus)

	t.Run("ok", func(t *testing.T) {
		var capturedBody []byte
		var capturedHeaders http.Header
		mux := http.NewServeMux()
		mux.Handle("POST /", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			capturedHeaders = request.Header
			capturedBody, _ = io.ReadAll(request.Body)
			writer.WriteHeader(http.StatusOK)
		}))
		subscriberServer := httptest.NewServer(mux)
		channel := RestHookChannel{
			Endpoint: subscriberServer.URL,
			Client:   subscriberServer.Client(),
		}
		expectedJSON, _ := json.Marshal(notification)

		err := channel.Notify(context.Background(), notification)

		require.NoError(t, err)
		require.Equal(t, fhirclient.FhirJsonMediaType, capturedHeaders.Get("Content-Type"))
		require.JSONEq(t, string(expectedJSON), string(capturedBody))
	})
	t.Run("non-OK status code", func(t *testing.T) {
		subscriberServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
		}))
		channel := RestHookChannel{
			Endpoint: subscriberServer.URL,
			Client:   subscriberServer.Client(),
		}

		err := channel.Notify(context.Background(), notification)

		require.ErrorIs(t, err, ReceiverFailure)
		require.EqualError(t, err, "FHIR subscription could not be delivered to receiver\nnon-OK HTTP response status: 404 Not Found")
	})
	t.Run("subscriber endpoint unreachable", func(t *testing.T) {
		subscriberServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusNotFound)
		}))
		subscriberServer.Close()
		channel := RestHookChannel{
			Endpoint: subscriberServer.URL,
			Client:   subscriberServer.Client(),
		}

		err := channel.Notify(context.Background(), notification)

		require.ErrorIs(t, err, ReceiverFailure)
		require.ErrorContains(t, err, "connection refused")
	})
}
