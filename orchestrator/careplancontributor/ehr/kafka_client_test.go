package ehr

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestKafkaClient_SubmitMessage(t *testing.T) {
	ctx := context.Background()
	type testStruct struct {
		name          string
		config        *KafkaConfig
		key           string
		value         string
		expectedError error
		setup         func(tt *testStruct)
		teardown      func()
	}
	tests := []testStruct{
		{
			name: "successful message submission",
			config: &KafkaConfig{
				Enabled: false,
			},
			key:   "test-key",
			value: "test-value",
		},
		{
			name: "error creating client - demo mode",
			config: &KafkaConfig{
				Enabled: true,
				Demo:    true,
			},
			expectedError: errors.New("demo mode is not allowed at the same time as strict mode"),
			setup: func(tt *testStruct) {
				globals.StrictMode = true
			},
		},
		{
			name: "demo mode client",
			config: &KafkaConfig{
				Enabled:  true,
				Demo:     true,
				Endpoint: "",
			},
			key:   "test-key",
			value: "test-value",
			setup: func(tt *testStruct) {
				globals.StrictMode = false
				// Create a test server
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Check the request method and URL
					require.Equal(t, "POST", r.Method)
					require.Equal(t, "/test-endpoint", r.URL.Path)

					// Read the request body
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.Equal(t, "test-value", string(body))

					// Respond with a success status
					w.WriteHeader(http.StatusOK)
				}))

				// Set the endpoint to the test server URL
				tt.config.Endpoint = ts.URL + "/test-endpoint"

				// Store the test server in the test struct for later teardown
				tt.teardown = func() {
					ts.Close()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			if tt.setup != nil {
				tt.setup(&tt)
			}

			client, err := NewClient(*tt.config)
			if err != nil {
				require.EqualError(t, err, tt.expectedError.Error())
				return
			}

			err = client.SubmitMessage(ctx, tt.key, tt.value)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			if tt.teardown != nil {
				tt.teardown()
			}
		})
	}
}
