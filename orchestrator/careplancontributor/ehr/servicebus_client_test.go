package ehr

import (
	"context"
	"errors"
	"go.uber.org/mock/gomock"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

	tests := []struct {
		name           string
		mockConnectErr error
		mockSendErr    error
		expectErr      bool
		setupMocks     func(mock *MockSbClient) func()
	}{
		{
			"Successful submit",
			nil,
			nil,
			false,
			func(mock *MockSbClient) func() {
				var old = newServiceBusClient
				newAzureServiceBusClient = func(config ServiceBusConfig) (ServiceBusClientWrapper, error) {
					return mock, nil
				}
				mock.EXPECT().SendMessage(ctx, gomock.Any()).Return(nil)
				mock.EXPECT().Close(ctx).Times(1)
				return func() {
					newServiceBusClient = old
				}
			},
		},
		{
			"Connect error", errors.New("connection failed"),
			nil,
			true,
			func(mock *MockSbClient) func() {
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
			"SendMessage error", nil, errors.New("send failed"), true, func(mock *MockSbClient) func() {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSbClient := NewMockSbClient(ctrl)

			client := &ServiceBusClientImpl{
				config: ServiceBusConfig{Topic: "test-topic"},
			}

			// Mock Connect method
			f := tt.setupMocks(mockSbClient)
			defer f()

			err := client.SubmitMessage(ctx, "key", "value")

			if (err != nil) != tt.expectErr {
				t.Fatalf("Expected error: %v, got: %v", tt.expectErr, err)
			}
		})
	}
}
