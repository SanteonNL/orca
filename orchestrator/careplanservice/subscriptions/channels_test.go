package subscriptions

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/pubsub"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
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
		require.EqualError(t, err, "FHIR subscription could not be delivered to receiver\nnon-OK HTTP response from "+subscriberServer.URL+" status: 404 Not Found")
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

func TestInProcessChannelFactory(t *testing.T) {
	ctx := tenants.WithTenant(context.Background(), tenants.Test().Sole())
	t.Run("local identity, in-process channel can be used", func(t *testing.T) {
		factory := InProcessChannelFactory{
			Profile: profile.Test(),
		}

		notificationsReceived := 0
		capturedPrincipal := auth.Principal{}
		pubsub.DefaultSubscribers.FhirSubscriptionNotify = func(ctx context.Context, _ any) error {
			var err error
			capturedPrincipal, err = auth.PrincipalFromContext(ctx)
			require.NoError(t, err)
			notificationsReceived++
			return nil
		}

		channel, err := factory.Create(ctx, auth.TestPrincipal1.Organization.Identifier[0])

		require.NoError(t, err)
		require.NotNil(t, channel)

		err = channel.Notify(ctx, coolfhir.SubscriptionNotification{})
		require.NoError(t, err)
		require.Equal(t, 1, notificationsReceived)
		require.Equal(t, auth.TestPrincipal1.Organization, capturedPrincipal.Organization)
	})
	t.Run("no local identities match, in-process channel cannot be used", func(t *testing.T) {
		prof := profile.TestProfile{
			Principal: auth.TestPrincipal2,
		}
		defaultChannelFactory := &stubChannelFactory{}
		factory := InProcessChannelFactory{
			Profile:               prof,
			DefaultChannelFactory: defaultChannelFactory,
		}

		expected := auth.TestPrincipal1.Organization.Identifier[0]
		_, err := factory.Create(ctx, expected)

		require.NoError(t, err)
		require.Len(t, defaultChannelFactory.calls, 1)
		require.Equal(t, expected, defaultChannelFactory.calls[0])
	})
	t.Run("no local identities, in-process channel cannot be used", func(t *testing.T) {
		prof := profile.TestProfile{
			Principal: &auth.Principal{
				Organization: fhir.Organization{
					Identifier: []fhir.Identifier{},
				},
			},
		}
		defaultChannelFactory := &stubChannelFactory{}
		factory := InProcessChannelFactory{
			Profile:               prof,
			DefaultChannelFactory: defaultChannelFactory,
		}

		_, err := factory.Create(ctx, fhir.Identifier{})

		require.NoError(t, err)
		require.Len(t, defaultChannelFactory.calls, 1)
		require.Equal(t, fhir.Identifier{}, defaultChannelFactory.calls[0])
	})

}

type stubChannelFactory struct {
	calls []fhir.Identifier
}

func (s *stubChannelFactory) Create(ctx context.Context, subscriber fhir.Identifier) (Channel, error) {
	s.calls = append(s.calls, subscriber)
	return nil, nil
}
