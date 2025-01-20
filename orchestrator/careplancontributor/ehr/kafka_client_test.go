package ehr

import (
	"context"
	"errors"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/mock/gomock"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock struct for KafkaClientImpl
type MockKafkaClientImpl struct {
	mock.Mock
}

func (m *MockKafkaClientImpl) SubmitMessage(ctx context.Context, key string, value string) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func TestNewClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	tests := []struct {
		name        string
		config      KafkaConfig
		wantType    KafkaClient
		expectError bool
		prepare     func() func()
	}{
		{
			name: "DebugClientDebugOnlyEnabled",
			config: KafkaConfig{
				Enabled:   true,
				DebugOnly: true,
			},
			wantType:    &DebugClient{},
			expectError: false,
		},
		{
			name: "Plain",
			config: KafkaConfig{
				Enabled: true,
			},
			wantType:    &KafkaClientImpl{},
			expectError: false,
		},
		{
			name: "KafkaClientImplEnabled",
			config: KafkaConfig{
				Enabled:       true,
				DebugOnly:     false,
				PingOnStartup: false,
			},
			wantType:    &MockKafkaClient{},
			expectError: false,
			prepare: func() func() {
				mockKafkaClient := NewMockKafkaClient(ctrl)
				var originalNewKafkaClient = newKafkaClient
				newKafkaClient = func(config KafkaConfig) KafkaClient {
					return mockKafkaClient
				}
				return func() {
					newKafkaClient = originalNewKafkaClient
				}
			},
		},
		{
			name: "KafkaClientImplEnabledPingOnStartup",
			config: KafkaConfig{
				Enabled:       true,
				DebugOnly:     false,
				PingOnStartup: true,
			},
			wantType:    &MockKafkaClient{},
			expectError: false,
			prepare: func() func() {
				mockKafkaClient := NewMockKafkaClient(ctrl)
				mockKafkaClient.EXPECT().PingConnection(gomock.Any()).Return(nil)
				var originalNewKafkaClient = newKafkaClient
				newKafkaClient = func(config KafkaConfig) KafkaClient {
					return mockKafkaClient
				}
				return func() {
					newKafkaClient = originalNewKafkaClient
				}
			},
		},
		{
			name: "KafkaClientImplEnabledPingOnStartup",
			config: KafkaConfig{
				Enabled:       true,
				DebugOnly:     false,
				PingOnStartup: true,
			},
			wantType:    &MockKafkaClient{},
			expectError: true,
			prepare: func() func() {
				mockKafkaClient := NewMockKafkaClient(ctrl)
				mockKafkaClient.EXPECT().PingConnection(gomock.Any()).Return(errors.New("mock error"))
				var originalNewKafkaClient = newKafkaClient
				newKafkaClient = func(config KafkaConfig) KafkaClient {
					return mockKafkaClient
				}
				return func() {
					newKafkaClient = originalNewKafkaClient
				}
			},
		},
		{
			name: "NoopClientWhenDisabled",
			config: KafkaConfig{
				Enabled: false,
			},
			wantType:    &NoopClient{},
			expectError: false,
		},
		{
			name: "DemoClient",
			config: KafkaConfig{
				Enabled: true,
				Demo:    true,
			},
			wantType:    &DemoClient{},
			expectError: false,
		},
		{
			name: "DemoClient - StrictMode - Not allowed",
			config: KafkaConfig{
				Enabled: true,
				Demo:    true,
			},
			wantType:    &DemoClient{},
			expectError: true,
			prepare: func() func() {
				globals.StrictMode = true
				return func() {
					globals.StrictMode = true
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepare := tt.prepare
			if prepare != nil {
				f := prepare()
				defer f()
			}

			client, err := NewClient(tt.config)
			if (err != nil) != tt.expectError {
				t.Errorf("NewClient() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !tt.expectError {
				switch tt.wantType.(type) {
				case *DebugClient:
					_, ok := client.(*DebugClient)
					assert.True(t, ok, "Expected DebugClient but got something else")
				case *KafkaClientImpl:
					_, ok := client.(*KafkaClientImpl)
					assert.True(t, ok, "Expected KafkaClientImpl but got something else")
				case *MockKafkaClient:
					_, ok := client.(*MockKafkaClient)
					assert.True(t, ok, "Expected KafkaClientImpl but got something else")
				case *NoopClient:
					_, ok := client.(*NoopClient)
					assert.True(t, ok, "Expected NoopClient but got something else")
				case *DemoClient:
					_, ok := client.(*DemoClient)
					assert.True(t, ok, "Expected DemoClient but got something else")
				default:
					t.Errorf("Unexpected client type")
				}
			}
		})
	}
}

func TestSubmitMessage(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	type testStruct struct {
		name        string
		config      KafkaConfig
		expectError bool
		mockSetup   func(tt *testStruct) func(tt *testStruct) // Optional setup and teardown for mocks
	}
	tests := []testStruct{
		{
			name: "DisabledClient",
			config: KafkaConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "DebugClientDebugOnlyEnabled",
			config: KafkaConfig{
				Enabled:   true,
				DebugOnly: true,
			},
			expectError: false,
		},
		{
			name: "DebugClientDebugOnlyEnabled",
			config: KafkaConfig{
				Enabled:       true,
				DebugOnly:     true,
				PingOnStartup: true,
			},
			expectError: false,
		},
		{
			name: "SendingMessage",
			config: KafkaConfig{
				Enabled: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
					kgo.ProduceResult{
						Record: &kgo.Record{},
					},
				})
				mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}
			},
			expectError: false,
		},
		{
			name: "SendingMessageWithPing",
			config: KafkaConfig{
				Enabled:       true,
				PingOnStartup: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
					kgo.ProduceResult{
						Record: &kgo.Record{},
					},
				})
				mockKgoClient.EXPECT().Ping(ctx).Return(nil)
				mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}
			},
			expectError: false,
		},
		{
			name: "NewFail",
			config: KafkaConfig{
				Enabled: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return nil, errors.New("mock error")
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}
			},
			expectError: true,
		},
		{
			name: "PingFail",
			config: KafkaConfig{
				Enabled:       true,
				PingOnStartup: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().Ping(ctx).Return(errors.New("mock error"))
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}

			},
			expectError: true,
		},
		{
			name: "ErrorSendingMessage",
			config: KafkaConfig{
				Enabled: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
					kgo.ProduceResult{
						Record: nil,
						Err:    errors.New("mock error"),
					},
				})
				//mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}
			},
			expectError: true,
		},
		{
			name: "FlushError",
			config: KafkaConfig{
				Enabled: true,
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
					kgo.ProduceResult{
						Record: &kgo.Record{},
					},
				})
				mockKgoClient.EXPECT().Flush(ctx).Return(errors.New("mock error"))
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}
				return func(tt *testStruct) {
					newKgoClient = org
				}
			},
			expectError: true,
		},
		{
			name: "OauthBarerError1",
			config: KafkaConfig{
				Enabled: true,
				Sasl: SaslConfig{
					Mechanism: "OAUTHBEARER",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				// Standard mock
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				//mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
				//	kgo.ProduceResult{
				//		Record: &kgo.Record{},
				//	},
				//})
				//mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}

				// Azure mock
				var orgOathClient = newAzureOauthClient
				mockClient := NewMockAzureOauthClient(ctrl)
				mockClient.EXPECT().GetAzureCredential().Return(nil, errors.New("mock error"))
				newAzureOauthClient = func() (AzureOauthClient, error) {
					return mockClient, nil
				}
				return func(tt *testStruct) {
					newAzureOauthClient = orgOathClient
					newKgoClient = org
				}
			},
			expectError: true,
		},
		{
			name: "OauthBarerError2",
			config: KafkaConfig{
				Enabled:  true,
				Endpoint: "https://example.com",
				Sasl: SaslConfig{
					Mechanism: "OAUTHBEARER",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				// Standard mock
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				//mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
				//	kgo.ProduceResult{
				//		Record: &kgo.Record{},
				//	},
				//})
				//mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}

				// Azure mock
				var orgOathClient = newAzureOauthClient
				mockClient := NewMockAzureOauthClient(ctrl)
				credential := azidentity.DefaultAzureCredential{}
				mockClient.EXPECT().GetAzureCredential().Return(&credential, nil)
				mockClient.EXPECT().GetBearerToken(ctx, &credential, "https://example.com").Return(nil, errors.New("mock error"))
				newAzureOauthClient = func() (AzureOauthClient, error) {
					return mockClient, nil
				}
				return func(tt *testStruct) {
					newAzureOauthClient = orgOathClient
					newKgoClient = org
				}
			},
			expectError: true,
		},
		{
			name: "OauthBarer",
			config: KafkaConfig{
				Enabled:  true,
				Endpoint: "https://example.com",
				Sasl: SaslConfig{
					Mechanism: "OAUTHBEARER",
				},
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT",
				},
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {
				// Standard mock
				var org = newKgoClient
				mockKgoClient := NewMockKgoClient(ctrl)
				mockKgoClient.EXPECT().ProduceSync(ctx, gomock.Any()).Return([]kgo.ProduceResult{
					kgo.ProduceResult{
						Record: &kgo.Record{},
					},
				})
				mockKgoClient.EXPECT().Flush(ctx).Return(nil)
				newKgoClient = func(opts []kgo.Opt) (KgoClient, error) {
					return mockKgoClient, nil
				}

				// Azure mock
				var orgOathClient = newAzureOauthClient
				mockClient := NewMockAzureOauthClient(ctrl)
				credential := azidentity.DefaultAzureCredential{}
				token := azcore.AccessToken{}
				mockClient.EXPECT().GetAzureCredential().Return(&credential, nil)
				mockClient.EXPECT().GetBearerToken(ctx, &credential, "https://example.com").Return(&token, nil)
				newAzureOauthClient = func() (AzureOauthClient, error) {
					return mockClient, nil
				}
				return func(tt *testStruct) {
					newAzureOauthClient = orgOathClient
					newKgoClient = org
				}
			},
			expectError: false,
		},
		{
			name: "demo mode client",
			config: KafkaConfig{
				Enabled: true,
				Demo:    true,
			},
			mockSetup: func(tt *testStruct) func(tt *testStruct) {

				globals.StrictMode = false
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.Equal(t, "/test-endpoint", r.URL.Path)

					body, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					require.Equal(t, "value", string(body))

					w.WriteHeader(http.StatusOK)
				}))

				tt.config.Endpoint = ts.URL + "/test-endpoint"

				return func(tt *testStruct) {
					ts.Close()
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock if defined
			if tt.mockSetup != nil {
				teardown := tt.mockSetup(&tt)
				defer teardown(&tt)
			}

			kafkaClient, err := NewClient(tt.config)
			if err == nil {
				err = kafkaClient.SubmitMessage(ctx, "key", "value")
			}
			if tt.expectError {
				assert.Error(t, err, "Expected error but got none")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}

func TestDebugClientSubmitMessage(t *testing.T) {
	ctx := context.Background()
	key := "debugKey"
	value := "debugValue"
	debugClient := DebugClient{}

	err := debugClient.SubmitMessage(ctx, key, value)
	assert.NoError(t, err, "SubmitMessage should not return an error")

	expectedFilePath := os.TempDir() + "/" + key + ".json"
	_, statErr := os.Stat(expectedFilePath)
	assert.NoError(t, statErr, "Debug file should exist")

	_ = os.Remove(expectedFilePath)
}

func TestNoopClientSubmitMessage(t *testing.T) {
	ctx := context.Background()
	key := "noopKey"
	value := "noopValue"
	noopClient := NoopClient{}

	err := noopClient.SubmitMessage(ctx, key, value)
	assert.NoError(t, err, "SubmitMessage should not return an error")
}

func TestGetAccessTokenErr(t *testing.T) {
	ctx := context.Background()
	endpoint := "testEndpoint"

	// Mock newAzureOauthClient behavior to simulate failure
	oauthClientError := errors.New("mock OAuth client error")
	newAzureOauthClient = func() (AzureOauthClient, error) {
		return nil, oauthClientError
	}

	_, err := getAccessToken(ctx, endpoint)
	assert.Error(t, err, "Expected error when getting access token")
	assert.Equal(t, oauthClientError, err, "Expected mock error")
}

func TestDebugClient(t *testing.T) {
	ctx := context.Background()
	client := DebugClient{}
	err := client.PingConnection(ctx)
	if err == nil {
		err = client.SubmitMessage(ctx, "key", "value")
	}

	assert.NoError(t, err, "Unexpected error when getting access token")
}

func TestNoopClient(t *testing.T) {
	ctx := context.Background()
	client := NoopClient{}
	err := client.PingConnection(ctx)
	if err == nil {
		err = client.SubmitMessage(ctx, "key", "value")
	}

	assert.NoError(t, err, "Unexpected error when getting access token")
}

func TestGetAccessToken(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	endpoint := "testEndpoint"
	token := azcore.AccessToken{
		Token:     "token",
		ExpiresOn: time.Time{},
	}

	// Mock newAzureOauthClient behavior to simulate failure
	mockAzureOauthClient := NewMockAzureOauthClient(ctrl)
	mockAzureOauthClient.EXPECT().GetAzureCredential().Return(nil, nil)
	mockAzureOauthClient.EXPECT().GetBearerToken(ctx, nil, endpoint).Return(&token, nil)
	newAzureOauthClient = func() (AzureOauthClient, error) {
		return mockAzureOauthClient, nil
	}

	foundToken, err := getAccessToken(ctx, endpoint)
	assert.NoError(t, err, "Not expected error when getting access token")
	assert.Equal(t, foundToken.Token, "token", "Expected token")
}

func TestKafkaClientImpl_Connect(t *testing.T) {
	tests := []struct {
		name        string
		config      KafkaConfig
		expectError bool
		mockSetup   func() func() // Optional setup and teardown for mocks
	}{
		{
			name: "UnsupportedProtocol",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "SASL_SSL", // Unknown protocol
				},
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
			},
			expectError: false,
		},
		{
			name: "UnsupportedProtocol",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT", // Unknown protocol
				},
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
			},
			expectError: false,
		},
		{
			name: "UnsupportedProtocol",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "UNKNOWN", // Unknown protocol
				},
				Sasl: SaslConfig{
					Mechanism: "PLAIN",
				},
			},
			expectError: true,
		},
		{
			name: "UnsupportedMechanism",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "SASL_SSL", // Unknown protocol
				},
				Sasl: SaslConfig{
					Mechanism: "UNKNOWN",
				},
			},
			expectError: true,
		},
		{
			name: "UnsupportedMechanism",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "SASL_PLAINTEXT", // Unknown protocol
				},
				Sasl: SaslConfig{
					Mechanism: "UNKNOWN",
				},
			},
			expectError: true,
		},
		{
			name: "SASL_SSL_ConnectSuccess",
			config: KafkaConfig{
				Security: SecurityConfig{
					Protocol: "SASL_SSL",
				},
				Sasl: SaslConfig{
					Mechanism: "OAUTHBEARER",
				},
			},
			expectError: false,
			mockSetup: func() func() {
				originalgetAccessToken := getAccessToken
				getAccessToken = func(ctx context.Context, endpoint string) (*azcore.AccessToken, error) {
					return &azcore.AccessToken{
						Token:     "token",
						ExpiresOn: time.Now().Add(time.Hour),
					}, nil
				}
				return func() {
					getAccessToken = originalgetAccessToken
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock if defined
			if tt.mockSetup != nil {
				teardown := tt.mockSetup()
				defer teardown()
			}

			kafkaClient := KafkaClientImpl{config: tt.config}
			_, err := kafkaClient.Connect(context.Background())

			if tt.expectError {
				assert.Error(t, err, "Expected error but got none")
			} else {
				assert.NoError(t, err, "Unexpected error")
			}
		})
	}
}
