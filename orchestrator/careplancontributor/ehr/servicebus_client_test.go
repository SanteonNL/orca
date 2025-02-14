package ehr

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name       string
		config     ServiceBusConfig
		expectType ServiceBusClient
		expectErr  bool
		setup      func() func()
	}{
		{
			name: "NoopClient when disabled",
			config: ServiceBusConfig{
				Enabled: false,
			},
			expectType: &NoopClient{},
			expectErr:  false,
		},
		{
			name: "DebugClient when debug only",
			config: ServiceBusConfig{
				Enabled:   true,
				DebugOnly: true,
			},
			expectType: &DebugClient{},
			expectErr:  false,
		},
		{
			name: "DemoClient when demo endpoint is set",
			config: ServiceBusConfig{
				Enabled:      true,
				DebugOnly:    false,
				DemoEndpoint: "http://localhost:8080",
			},
			expectType: &DemoClient{},
			expectErr:  false,
		},
		{
			name: "DemoClient when demo endpoint is set in strict mode",
			config: ServiceBusConfig{
				Enabled:      true,
				DebugOnly:    false,
				DemoEndpoint: "http://localhost:8080",
			},
			expectType: nil,
			expectErr:  true,
			setup: func() func() {
				globals.StrictMode = true
				return func() {
					globals.StrictMode = false
				}
			},
		},
		{
			name: "PingOnStartup enabled with incomplete config",
			config: ServiceBusConfig{
				Enabled:       true,
				DebugOnly:     false,
				PingOnStartup: true,
			},
			expectType: nil,
			expectErr:  true, // Expecting an error due to incomplete configuration
		},
		{
			name: "ServiceBusClientImpl with valid config",
			config: ServiceBusConfig{
				Enabled:   true,
				DebugOnly: false,
			},
			expectType: &ServiceBusClientImpl{},
			expectErr:  false,
		},
		{
			name: "ServiceBusClientImpl with valid config and demo endpoint set",
			config: ServiceBusConfig{
				Enabled:      true,
				DebugOnly:    false,
				DemoEndpoint: "http://localhost:8080",
			},
			expectType: &ServiceBusClientImpl{},
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := tt.setup
			if setup != nil {
				f := setup()
				defer f()
			}
			client, err := NewClient(tt.config)

			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}

			if tt.expectType != nil {
				if _, ok := client.(ServiceBusClient); !ok {
					t.Fatalf("Expected client of type %T, got another type", tt.expectType)
				}
			}
		})
	}
}

func TestDebugClient_SubmitMessage(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		value       string
		expectErr   bool
		fileCreated bool
	}{
		{"Valid debug message", "test-key", "test-value", false, true},
		{"Invalid debug message key with path separator", "test/key", "test-value", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &DebugClient{}
			err := client.SubmitMessage(context.Background(), tt.key, tt.value)

			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}

			if tt.fileCreated {
				expectedPath := filepath.Join(os.TempDir(), strings.ReplaceAll(tt.key, ":", "_")+".json")
				_, statErr := os.Stat(expectedPath)
				if statErr != nil {
					t.Fatalf("Expected file was not created at %s: %v", expectedPath, statErr)
				}
				_ = os.Remove(expectedPath) // Cleanup
			}
		})
	}
}

func TestNoopClient_SubmitMessage(t *testing.T) {
	client := &NoopClient{}
	err := client.SubmitMessage(context.Background(), "test", "test-value")

	if err != nil {
		t.Fatalf("SubmitMessage failed with error: %v", err)
	}
}

func TestServiceBusClientImpl_SubmitMessage(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	type testStruct struct {
		name           string
		mockConnectErr error
		mockSendErr    error
		expectErr      bool
		config         *ServiceBusConfig
		setupMocks     func(mock *MockServiceBusClientWrapper, tt *testStruct) func()
	}

	tests := []testStruct{
		{
			name:           "Successful submit",
			mockConnectErr: nil,
			mockSendErr:    nil,
			expectErr:      false,
			setupMocks: func(mock *MockServiceBusClientWrapper, tt *testStruct) func() {
				var old = newServiceBusClient
				newAzureServiceBusClient = func(config ServiceBusConfig) (ServiceBusClientWrapper, error) {
					return mock, nil
				}
				mock.EXPECT().SendMessage(ctx, gomock.Any()).DoAndReturn(func(ctx context.Context, message *azservicebus.Message) error {
					require.Equal(t, "key", *message.CorrelationID)
					require.Equal(t, "value", string(message.Body))
					require.Equal(t, "application/json", *message.ContentType)
					return nil
				})
				mock.EXPECT().Close(ctx).Times(1)
				return func() {
					newServiceBusClient = old
				}
			},
		},
		{
			name:           "Connect error",
			mockConnectErr: errors.New("connection failed"),
			mockSendErr:    nil,
			expectErr:      true,
			setupMocks: func(mock *MockServiceBusClientWrapper, tt *testStruct) func() {
				var old = newServiceBusClient
				newAzureServiceBusClient = func(config ServiceBusConfig) (ServiceBusClientWrapper, error) {
					return nil, errors.New("connection failed")
				}
				return func() {
					newServiceBusClient = old
				}
			},
		},
		{
			name:           "SendMessage error",
			mockConnectErr: nil,
			mockSendErr:    errors.New("send failed"),
			expectErr:      true,
			setupMocks: func(mock *MockServiceBusClientWrapper, tt *testStruct) func() {
				var old = newServiceBusClient
				newAzureServiceBusClient = func(config ServiceBusConfig) (ServiceBusClientWrapper, error) {
					return mock, nil
				}
				mock.EXPECT().SendMessage(ctx, gomock.Any()).Return(errors.New("send failed"))
				mock.EXPECT().Close(ctx).Times(1)
				return func() {
					newServiceBusClient = old
				}

			},
		},
		{
			name:           "with demo client - success",
			expectErr:      false,
			mockConnectErr: nil,
			mockSendErr:    nil,
			setupMocks: func(mock *MockServiceBusClientWrapper, tt *testStruct) func() {
				globals.StrictMode = false
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, "/test-endpoint", r.URL.Path)
					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.Equal(t, `{"value":"value"}`, string(body))
					w.WriteHeader(http.StatusOK)
				}))

				tt.config.DemoEndpoint = ts.URL + "/test-endpoint"

				var old = newServiceBusClient
				newAzureServiceBusClient = func(config ServiceBusConfig) (ServiceBusClientWrapper, error) {
					return mock, nil
				}
				mock.EXPECT().SendMessage(ctx, gomock.Any()).Return(nil)
				mock.EXPECT().Close(ctx).Times(1)
				return func() {
					newServiceBusClient = old
					ts.Close()
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSbClient := NewMockServiceBusClientWrapper(ctrl)

			if tt.config == nil {
				tt.config = &ServiceBusConfig{Topic: "test-topic"}
			}
			client := &ServiceBusClientImpl{
				config: *tt.config,
			}

			// Mock Connect method
			f := tt.setupMocks(mockSbClient, &tt)
			defer f()

			err := client.SubmitMessage(ctx, "key", "value")

			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}
