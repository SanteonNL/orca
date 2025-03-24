package webhook

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/messaging"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	events "github.com/SanteonNL/orca/orchestrator/events"
	"github.com/stretchr/testify/require"
)

var _ events.Type = StringEvent("")

type StringEvent string

func (s StringEvent) Topic() messaging.Topic {
	return messaging.Topic{
		Name: "string-event",
	}
}

func (s StringEvent) Instance() events.Type {
	return StringEvent("")
}

var _ events.Type = &unmarshallable{}

type unmarshallable struct{}

func (u unmarshallable) Topic() messaging.Topic {
	return messaging.Topic{}
}

func (u unmarshallable) Instance() events.Type {
	return unmarshallable{}
}

func (u unmarshallable) MarshalJSON() ([]byte, error) {
	return nil, errors.New("fail")
}

func TestEventHandler_Handle(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.TODO())
	cancel()

	tests := map[string]struct {
		event         events.Type
		expectedError string
		context       context.Context
		handlerFunc   http.HandlerFunc
	}{
		"happy path": {
			event:   StringEvent("noop"),
			context: context.TODO(),
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				defer r.Body.Close()

				data, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				require.Equal(t, "\"noop\"", string(data))
				require.Equal(t, "application/json", r.Header.Get("Content-Type"))

				w.WriteHeader(http.StatusOK)
			},
		},
		"context canceled": {
			event:         StringEvent("noop"),
			context:       cancelled,
			expectedError: "context canceled",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
			},
		},
		"failed to marshal": {
			event:         unmarshallable{},
			context:       context.TODO(),
			expectedError: "failed to serialize event",
			handlerFunc:   func(w http.ResponseWriter, r *http.Request) {},
		},
		"failed to create request": {
			event:         StringEvent("noop"),
			context:       nil,
			expectedError: "failed to create request",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
			},
		},
		"failed to send request": {
			event:         StringEvent("noop"),
			context:       context.TODO(),
			expectedError: "failed to send request",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				panic("stop")
			},
		},
		"unexpected HTTP status": {
			event:         StringEvent("noop"),
			context:       context.TODO(),
			expectedError: "unexpected status code: 500",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handlerFunc))
			defer server.Close()

			handler := EventHandler{
				client: server.Client(),
				URL:    server.URL,
			}

			err := handler.Handle(tt.context, tt.event)

			if tt.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.expectedError)
			}
		})
	}
}
