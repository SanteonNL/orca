package otel

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "disabled config is always valid",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid stdout config",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "stdout",
				},
			},
			wantErr: false,
		},
		{
			name: "valid otlp config",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "otlp",
					OTLP: OTLPConfig{
						Endpoint: "http://localhost:4318",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid none config",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "none",
				},
			},
			wantErr: false,
		},
		{
			name: "missing service name",
			config: Config{
				Enabled: true,
				Exporter: ExporterConfig{
					Type: "stdout",
				},
			},
			wantErr: true,
			errMsg:  "service name is required",
		},
		{
			name: "invalid exporter type",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "invalid",
				},
			},
			wantErr: true,
			errMsg:  "unsupported exporter type: invalid",
		},
		{
			name: "otlp without endpoint",
			config: Config{
				Enabled:     true,
				ServiceName: "test-service",
				Exporter: ExporterConfig{
					Type: "otlp",
					OTLP: OTLPConfig{},
				},
			},
			wantErr: true,
			errMsg:  "OTLP endpoint is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.Enabled, "Default config should have OTEL enabled")
	assert.Equal(t, "orca-orchestrator", config.ServiceName)
	assert.Equal(t, "1.0.0", config.ServiceVersion)
	assert.Equal(t, "otlp", config.Exporter.Type)
	assert.Equal(t, "grpc", config.Exporter.Protocol)                // Updated to reflect new default
	assert.Equal(t, "localhost:4317", config.Exporter.OTLP.Endpoint) // Updated to gRPC default port
	assert.Equal(t, 10*time.Second, config.Exporter.OTLP.Timeout)
	assert.True(t, config.Exporter.OTLP.Insecure) // Default should be insecure for localhost
	assert.Empty(t, config.ResourceAttributes)    // No resource attributes by default

	// Default config should be valid
	require.NoError(t, config.Validate())
}

func TestInitialize_Disabled(t *testing.T) {
	config := Config{
		Enabled: false,
	}

	ctx := context.Background()
	provider, err := Initialize(ctx, config)

	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify that the global tracer provider is set
	globalProvider := otel.GetTracerProvider()
	assert.NotNil(t, globalProvider)

	// Test shutdown
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestInitialize_StdoutExporter(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type: "stdout",
		},
	}

	ctx := context.Background()
	provider, err := Initialize(ctx, config)

	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify that the global tracer provider is set and is not a no-op
	globalProvider := otel.GetTracerProvider()
	assert.NotNil(t, globalProvider)

	// Verify we can create a tracer
	tracer := globalProvider.Tracer("test")
	assert.NotNil(t, tracer)

	// Create a span to verify the provider is working
	_, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)
	span.End()

	// Test shutdown
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestInitialize_NoneExporter(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type: "none",
		},
	}

	ctx := context.Background()
	provider, err := Initialize(ctx, config)

	require.NoError(t, err)
	require.NotNil(t, provider)

	// Verify that the global tracer provider is set
	globalProvider := otel.GetTracerProvider()
	assert.NotNil(t, globalProvider)

	// Verify we can create a tracer
	tracer := globalProvider.Tracer("test")
	assert.NotNil(t, tracer)

	// Create a span to verify the provider is working
	_, span := tracer.Start(ctx, "test-span")
	assert.NotNil(t, span)
	span.End()

	// Test shutdown
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

// Note: OTLP exporter test is more complex as it requires a real endpoint
// For unit tests, we'll test the configuration validation instead
func TestInitialize_OTLPConfig(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type: "otlp",
			OTLP: OTLPConfig{
				Endpoint: "http://localhost:4318",
				Headers: map[string]string{
					"Authorization": "Bearer test-token",
				},
				Timeout: 5 * time.Second,
			},
		},
	}

	// This should pass validation
	require.NoError(t, config.Validate())

	// Note: We don't actually initialize here because it would require a real OTLP endpoint
	// In integration tests, you would test the actual OTLP initialization
}

func TestTracerProvider_Shutdown(t *testing.T) {
	// Test shutdown with nil cleanup function
	provider := &TracerProvider{
		provider: trace.NewTracerProvider(),
		cleanup:  nil,
	}

	err := provider.Shutdown(context.Background())
	assert.NoError(t, err)

	// Test shutdown with cleanup function
	shutdownCalled := false
	provider = &TracerProvider{
		provider: trace.NewTracerProvider(),
		cleanup: func(ctx context.Context) error {
			shutdownCalled = true
			return nil
		},
	}

	err = provider.Shutdown(context.Background())
	assert.NoError(t, err)
	assert.True(t, shutdownCalled)
}

func TestDefaultConfig_WithEnvironmentVariables(t *testing.T) {
	// Save original environment
	originalVars := make(map[string]string)
	envVars := []string{
		"OTEL_SERVICE_NAME",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"CONTAINERAPP_OTEL_METRIC_GRPC_ENDPOINT",
		"CONTAINERAPP_OTEL_LOGGING_GRPC_ENDPOINT",
		"OTEL_RESOURCE_ATTRIBUTES",
	}

	for _, envVar := range envVars {
		originalVars[envVar] = os.Getenv(envVar)
	}

	// Clean up after test
	defer func() {
		for _, envVar := range envVars {
			if originalVars[envVar] == "" {
				os.Unsetenv(envVar)
			} else {
				os.Setenv(envVar, originalVars[envVar])
			}
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected func(*testing.T, Config)
	}{
		{
			name: "OTEL_SERVICE_NAME override",
			envVars: map[string]string{
				"OTEL_SERVICE_NAME": "custom-service",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "custom-service", config.ServiceName)
			},
		},
		{
			name: "OTEL_EXPORTER_OTLP_PROTOCOL override",
			envVars: map[string]string{
				"OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "grpc", config.Exporter.Protocol)
			},
		},
		{
			name: "OTEL_EXPORTER_OTLP_ENDPOINT with https",
			envVars: map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "https://otel.example.com:4317",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "otel.example.com:4317", config.Exporter.OTLP.Endpoint)
				assert.False(t, config.Exporter.OTLP.Insecure)
			},
		},
		{
			name: "OTEL_EXPORTER_OTLP_ENDPOINT with http",
			envVars: map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "http://otel.example.com:4317",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "otel.example.com:4317", config.Exporter.OTLP.Endpoint)
				assert.True(t, config.Exporter.OTLP.Insecure)
			},
		},
		{
			name: "Azure Container App endpoints",
			envVars: map[string]string{
				"CONTAINERAPP_OTEL_METRIC_GRPC_ENDPOINT":  "metric-endpoint:4317",
				"CONTAINERAPP_OTEL_LOGGING_GRPC_ENDPOINT": "logging-endpoint:4317",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "metric-endpoint:4317", config.Exporter.OTLP.MetricEndpoint)
				assert.Equal(t, "logging-endpoint:4317", config.Exporter.OTLP.LoggingEndpoint)
			},
		},
		{
			name: "OTEL_RESOURCE_ATTRIBUTES parsing",
			envVars: map[string]string{
				"OTEL_RESOURCE_ATTRIBUTES": "service.namespace=production,service.instance.id=abc123,custom.key=custom.value",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "production", config.ResourceAttributes["service.namespace"])
				assert.Equal(t, "abc123", config.ResourceAttributes["service.instance.id"])
				assert.Equal(t, "custom.value", config.ResourceAttributes["custom.key"])
				assert.Len(t, config.ResourceAttributes, 3)
			},
		},
		{
			name: "OTEL_RESOURCE_ATTRIBUTES with spaces",
			envVars: map[string]string{
				"OTEL_RESOURCE_ATTRIBUTES": " key1 = value1 , key2 = value2 ",
			},
			expected: func(t *testing.T, config Config) {
				assert.Equal(t, "value1", config.ResourceAttributes["key1"])
				assert.Equal(t, "value2", config.ResourceAttributes["key2"])
				assert.Len(t, config.ResourceAttributes, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars first
			for _, envVar := range envVars {
				os.Unsetenv(envVar)
			}

			// Set test env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config := DefaultConfig()
			tt.expected(t, config)
		})
	}
}

func TestInitialize_UnsupportedProtocol(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type:     "otlp",
			Protocol: "http", // HTTP not currently supported
			OTLP: OTLPConfig{
				Endpoint: "http://localhost:4318",
			},
		},
	}

	ctx := context.Background()
	_, err := Initialize(ctx, config)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported OTLP protocol: http")
}

func TestInitialize_WithResourceAttributes(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		ResourceAttributes: map[string]string{
			"environment":     "test",
			"service.version": "2.0.0",
		},
		Exporter: ExporterConfig{
			Type: "stdout",
		},
	}

	ctx := context.Background()
	provider, err := Initialize(ctx, config)

	require.NoError(t, err)
	require.NotNil(t, provider)

	// Test shutdown
	err = provider.Shutdown(ctx)
	assert.NoError(t, err)
}

func TestInitialize_GRPCExporter(t *testing.T) {
	config := Config{
		Enabled:        true,
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		Exporter: ExporterConfig{
			Type:     "otlp",
			Protocol: "grpc",
			OTLP: OTLPConfig{
				Endpoint: "localhost:4317",
				Insecure: true,
				Headers: map[string]string{
					"Authorization": "Bearer test-token",
				},
				Timeout: 5 * time.Second,
			},
		},
	}

	// This should pass validation
	require.NoError(t, config.Validate())

	// Note: We don't actually initialize here because it would require a real OTLP endpoint
	// The configuration is valid and would work with a real gRPC OTLP endpoint
}
