package webhook

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	events "github.com/SanteonNL/orca/orchestrator/careplancontributor/event"
	"github.com/stretchr/testify/require"
)

type unmarshallable struct{}

func (u unmarshallable) MarshalJSON() ([]byte, error) {
	return nil, errors.New("fail")
}

func TestEventHandler_Handle(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.TODO())
	cancel()

	tests := map[string]struct {
		event         events.Instance
		expectedError string
		context       context.Context
		handlerFunc   http.HandlerFunc
	}{
		"happy path": {
			event:   events.Instance{FHIRResource: "noop"},
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
			event:         events.Instance{FHIRResource: "noop"},
			context:       cancelled,
			expectedError: "context canceled",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
			},
		},
		"failed to marshal": {
			event:         events.Instance{FHIRResource: unmarshallable{}},
			context:       context.TODO(),
			expectedError: "failed to serialize event",
			handlerFunc:   func(w http.ResponseWriter, r *http.Request) {},
		},
		"failed to create request": {
			event:         events.Instance{FHIRResource: "noop"},
			context:       nil,
			expectedError: "failed to create request",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
			},
		},
		"failed to send request": {
			event:         events.Instance{FHIRResource: "noop"},
			context:       context.TODO(),
			expectedError: "failed to send request",
			handlerFunc: func(w http.ResponseWriter, r *http.Request) {
				panic("stop")
			},
		},
		"unexpected HTTP status": {
			event:         events.Instance{FHIRResource: "noop"},
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
