package coolfhir

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewAuthRoundTripper_OpenTelemetry(t *testing.T) {
	// Test that OTEL instrumentation is properly applied without breaking functionality
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"resourceType": "Patient", "id": "test-patient"}`))
	}))
	defer testServer.Close()

	tests := []struct {
		name     string
		config   ClientConfig
		skipAuth bool
	}{
		{
			name: "default auth with otel instrumentation",
			config: ClientConfig{
				BaseURL: testServer.URL,
				Auth: AuthConfig{
					Type: Default,
				},
			},
			skipAuth: false,
		},
		{
			name: "azure managed identity config validation only",
			config: ClientConfig{
				BaseURL: testServer.URL,
				Auth: AuthConfig{
					Type:         AzureManagedIdentity,
					OAuth2Scopes: "https://test.com/.default",
				},
			},
			skipAuth: true, // Skip actual HTTP call due to auth issues in test env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create round tripper and FHIR client
			transport, fhirClient, err := NewAuthRoundTripper(tt.config, &fhirclient.Config{})

			if tt.skipAuth && err != nil {
				t.Skipf("Skipping Azure auth test in non-Azure environment: %v", err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, transport)
			require.NotNil(t, fhirClient)

			// Verify the transport is wrapped by otelhttp
			transportType := fmt.Sprintf("%T", transport)
			assert.Contains(t, transportType, "otelhttp", "Transport should be wrapped by otelhttp")

			if !tt.skipAuth {
				// Verify HTTP functionality still works with OTEL instrumentation
				ctx := context.Background()
				req, err := http.NewRequestWithContext(ctx, "GET", testServer.URL+"/Patient/test-patient", nil)
				require.NoError(t, err)

				resp, err := transport.RoundTrip(req)
				require.NoError(t, err)
				require.NotNil(t, resp)
				defer resp.Body.Close()

				// Verify the response works correctly
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Verify FHIR client functionality
				assert.NotNil(t, fhirClient, "FHIR client should be created successfully")
			}
		})
	}
}

func TestNewAuthRoundTripper_ConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  ClientConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid default config",
			config: ClientConfig{
				BaseURL: "http://example.com/fhir",
				Auth: AuthConfig{
					Type: Default,
				},
			},
			wantErr: false,
		},
		{
			name: "valid azure managed identity config",
			config: ClientConfig{
				BaseURL: "http://example.com/fhir",
				Auth: AuthConfig{
					Type:         AzureManagedIdentity,
					OAuth2Scopes: "https://example.com/.default",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid auth type",
			config: ClientConfig{
				BaseURL: "http://example.com/fhir",
				Auth: AuthConfig{
					Type: "invalid-auth-type",
				},
			},
			wantErr: true,
			errMsg:  "invalid FHIR authentication type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, fhirClient, err := NewAuthRoundTripper(tt.config, &fhirclient.Config{})

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, transport)
				assert.Nil(t, fhirClient)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, transport)
				assert.NotNil(t, fhirClient)
			}
		})
	}
}

func TestDefaultAzureScope(t *testing.T) {
	testURL, err := url.Parse("https://test-fhir.azurewebsites.net")
	require.NoError(t, err)

	scopes := DefaultAzureScope(testURL)
	expected := []string{"test-fhir.azurewebsites.net/.default"}

	assert.Equal(t, expected, scopes)
}

func TestNewAuthRoundTripper_OpenTelemetry_BasicSetup(t *testing.T) {
	// Just verify that the transport is created with OTEL instrumentation
	// without trying to capture spans (which is complex in tests)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"resourceType": "Patient", "id": "test-patient"}`))
	}))
	defer testServer.Close()

	tests := []struct {
		name          string
		config        ClientConfig
		shouldSucceed bool
	}{
		{
			name: "default auth creates instrumented transport",
			config: ClientConfig{
				BaseURL: testServer.URL,
				Auth: AuthConfig{
					Type: Default,
				},
			},
			shouldSucceed: true,
		},
		{
			name: "invalid auth type fails properly",
			config: ClientConfig{
				BaseURL: testServer.URL,
				Auth: AuthConfig{
					Type: "invalid-auth-type",
				},
			},
			shouldSucceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, fhirClient, err := NewAuthRoundTripper(tt.config, &fhirclient.Config{})

			if tt.shouldSucceed {
				require.NoError(t, err)
				require.NotNil(t, transport)
				require.NotNil(t, fhirClient)

				// Verify we can make a basic HTTP request
				req, err := http.NewRequestWithContext(context.Background(), "GET", testServer.URL+"/Patient/test", nil)
				require.NoError(t, err)

				resp, err := transport.RoundTrip(req)
				require.NoError(t, err)
				require.NotNil(t, resp)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				assert.Error(t, err)
				assert.Nil(t, transport)
				assert.Nil(t, fhirClient)
			}
		})
	}
}

func TestNewAuthRoundTripper_OpenTelemetry_Debug(t *testing.T) {
	// Create a minimal test to debug the OTEL instrumentation
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Server received request: %s %s", r.Method, r.URL.Path)
		w.Header().Set("Content-Type", "application/fhir+json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"resourceType": "Patient", "id": "test-patient"}`))
	}))
	defer testServer.Close()

	// Setup OTEL with a simple recorder
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	t.Logf("Initial spans count: %d", len(spanRecorder.Ended()))

	// Create the transport
	config := ClientConfig{
		BaseURL: testServer.URL,
		Auth: AuthConfig{
			Type: Default,
		},
	}

	transport, fhirClient, err := NewAuthRoundTripper(config, &fhirclient.Config{})
	require.NoError(t, err)
	require.NotNil(t, transport)
	require.NotNil(t, fhirClient)

	// Check if the transport is actually an otelhttp.Transport
	t.Logf("Transport type: %T", transport)

	// Make a direct HTTP request
	req, err := http.NewRequestWithContext(context.Background(), "GET", testServer.URL+"/Patient/test", nil)
	require.NoError(t, err)

	t.Logf("Making HTTP request to: %s", req.URL.String())

	// Use the transport directly
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Log what we got
	t.Logf("Response status: %d", resp.StatusCode)

	// Check spans immediately after request
	spansBeforeFlush := spanRecorder.Ended()
	t.Logf("Spans before flush: %d", len(spansBeforeFlush))

	// Force flush spans
	err = tracerProvider.ForceFlush(context.Background())
	require.NoError(t, err)

	// Check spans after flush
	spans := spanRecorder.Ended()
	t.Logf("Spans after flush: %d", len(spans))

	for i, span := range spans {
		t.Logf("Span %d: Name='%s', Kind=%v", i, span.Name(), span.SpanKind())
		attrs := span.Attributes()
		for _, attr := range attrs {
			t.Logf("  Attribute: %s = %s", attr.Key, attr.Value.AsString())
		}
	}

	// Try creating a manual span to verify the tracer provider works
	t.Logf("Creating manual span...")
	tracer := otel.GetTracerProvider().Tracer("test")
	_, manualSpan := tracer.Start(context.Background(), "manual-test-span")
	manualSpan.End()

	err = tracerProvider.ForceFlush(context.Background())
	require.NoError(t, err)

	allSpans := spanRecorder.Ended()
	t.Logf("Total spans after manual span: %d", len(allSpans))

	// Verify we have at least the manual span
	require.GreaterOrEqual(t, len(allSpans), 1, "Should have at least the manual span")

	// Look for the manual span
	foundManualSpan := false
	for _, span := range allSpans {
		if span.Name() == "manual-test-span" {
			foundManualSpan = true
			t.Logf("Found manual test span: %s", span.Name())
		}
	}
	require.True(t, foundManualSpan, "Should find the manual test span")
}

func TestNewAuthRoundTripper_TransportType(t *testing.T) {
	// Very simple test to check what type of transport we get
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	config := ClientConfig{
		BaseURL: testServer.URL,
		Auth: AuthConfig{
			Type: Default,
		},
	}

	transport, _, err := NewAuthRoundTripper(config, &fhirclient.Config{})
	require.NoError(t, err)
	require.NotNil(t, transport)

	// Check the actual type
	transportType := fmt.Sprintf("%T", transport)
	t.Logf("Actual transport type: %s", transportType)

	// The transport should be wrapped by otelhttp
	assert.Contains(t, transportType, "otelhttp", "Transport should be wrapped by otelhttp")
}

func TestClientConfig_Validate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		config := ClientConfig{
			BaseURL: "https://example.com/fhir",
		}
		err := config.Validate()
		require.NoError(t, err)
	})
	t.Run("no baseURL", func(t *testing.T) {
		config := ClientConfig{}
		err := config.Validate()
		require.NoError(t, err)
	})
}
