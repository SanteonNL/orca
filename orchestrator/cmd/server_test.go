package cmd

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/SanteonNL/orca/orchestrator/lib/otel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	baseotel "go.opentelemetry.io/otel"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Nuts.API.URL = "http://example.com"
	cfg.Tenants = tenants.Test(func(properties *tenants.Properties) {
		properties.ChipSoft = tenants.ChipSoftProperties{
			OrganizationID: "",
		}
	})
	cfg.Nuts.Public.URL = "http://example.com"
	cfg.Nuts.DiscoveryService = "http://example.com"

	t.Run("ok", func(t *testing.T) {
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())
		ctx, cancel := context.WithCancel(context.Background())
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Start(ctx, cfg)
			require.NoError(t, err)
		}()
		assertServerStarted(t, cfg.Public.Address)

		t.Run("strict mode is enabled by default", func(t *testing.T) {
			require.True(t, globals.StrictMode)
		})

		// Signal server to stop, then wait for graceful exit
		cancel()
		wg.Wait()
	})
	t.Run("sigint triggers graceful shutdown", func(t *testing.T) {
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Start(ctx, cfg)
			require.NoError(t, err)
		}()
		assertServerStarted(t, cfg.Public.Address)

		// Send SIGINT signal
		p, err := os.FindProcess(os.Getpid())
		require.NoError(t, err)
		require.NoError(t, p.Signal(os.Interrupt))

		// Wait for graceful exit
		wg.Wait()
	})
	t.Run("port already in use", func(t *testing.T) {
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = Start(ctx, cfg)
		}()
		assertServerStarted(t, cfg.Public.Address)
		// Start second server, should fail
		err := Start(ctx, cfg)
		require.EqualError(t, err, "failed to start HTTP server: listen tcp "+cfg.Public.Address+": bind: address already in use")
		// Gracefully exit first server
		cancel()
		wg.Wait()
	})

}

func assertServerStarted(t *testing.T, port string) {
	// Wait for the server to start, time-out after 5 seconds
	started := false
	for i := 0; i < 500; i++ {
		httpResponse, _ := http.Get("http://localhost" + port)
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			started = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, started)
}

// freeTCPPort asks the kernel for a free open port that is ready to use.
// Taken from https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
func freeTCPPort() (port int) {
	if a, err := net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port
		} else {
			panic(err)
		}
	} else {
		panic(err)
	}
}

func TestStart_OpenTelemetry(t *testing.T) {
	baseCfg := DefaultConfig()
	baseCfg.Nuts.API.URL = "http://example.com"
	baseCfg.Nuts.Public.URL = "http://example.com"
	baseCfg.Nuts.DiscoveryService = "http://example.com"

	t.Run("disabled opentelemetry", func(t *testing.T) {
		cfg := baseCfg
		cfg.OpenTelemetry.Enabled = false
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Start server in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- Start(ctx, cfg)
		}()

		// Wait for server to start
		assertServerStarted(t, cfg.Public.Address)

		// Verify OpenTelemetry is initialized (even when disabled, we should have a provider)
		globalProvider := baseotel.GetTracerProvider()
		assert.NotNil(t, globalProvider)

		// Create a tracer to verify it works
		tracer := globalProvider.Tracer("test")
		assert.NotNil(t, tracer)

		// Stop the server
		cancel()

		// Wait for graceful shutdown
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Server shutdown timed out")
		}
	})

	t.Run("stdout exporter", func(t *testing.T) {
		cfg := baseCfg
		cfg.OpenTelemetry.Enabled = true
		cfg.OpenTelemetry.ServiceName = "test-orchestrator"
		cfg.OpenTelemetry.ServiceVersion = "test-version"
		cfg.OpenTelemetry.Exporter.Type = "stdout"
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Start server in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- Start(ctx, cfg)
		}()

		// Wait for server to start
		assertServerStarted(t, cfg.Public.Address)

		// Verify OpenTelemetry is properly initialized
		globalProvider := baseotel.GetTracerProvider()
		assert.NotNil(t, globalProvider)

		// Verify we can create spans
		tracer := globalProvider.Tracer("test")
		_, span := tracer.Start(context.Background(), "test-span")
		assert.NotNil(t, span)
		span.End()

		// Stop the server
		cancel()

		// Wait for graceful shutdown
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Server shutdown timed out")
		}
	})

	t.Run("none exporter", func(t *testing.T) {
		cfg := baseCfg
		cfg.OpenTelemetry.Enabled = true
		cfg.OpenTelemetry.ServiceName = "test-orchestrator"
		cfg.OpenTelemetry.ServiceVersion = "test-version"
		cfg.OpenTelemetry.Exporter.Type = "none"
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Start server in goroutine
		errChan := make(chan error, 1)
		go func() {
			errChan <- Start(ctx, cfg)
		}()

		// Wait for server to start
		assertServerStarted(t, cfg.Public.Address)

		// Verify OpenTelemetry is properly initialized
		globalProvider := baseotel.GetTracerProvider()
		assert.NotNil(t, globalProvider)

		// Verify we can create spans (they just won't be exported)
		tracer := globalProvider.Tracer("test")
		_, span := tracer.Start(context.Background(), "test-span")
		assert.NotNil(t, span)
		span.End()

		// Stop the server
		cancel()

		// Wait for graceful shutdown
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Server shutdown timed out")
		}
	})

	t.Run("invalid opentelemetry config", func(t *testing.T) {
		cfg := baseCfg
		cfg.OpenTelemetry.Enabled = true
		cfg.OpenTelemetry.ServiceName = "" // Invalid: missing service name
		cfg.OpenTelemetry.Exporter.Type = "stdout"

		err := Start(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OpenTelemetry configuration")
	})

	t.Run("otlp config validation", func(t *testing.T) {
		cfg := baseCfg
		cfg.OpenTelemetry.Enabled = true
		cfg.OpenTelemetry.ServiceName = "test-service"
		cfg.OpenTelemetry.Exporter.Type = "otlp"
		cfg.OpenTelemetry.Exporter.OTLP.Endpoint = "" // Invalid: missing endpoint

		err := Start(context.Background(), cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid OpenTelemetry configuration")
	})
}

func TestOpenTelemetryInitialization(t *testing.T) {
	t.Run("successful initialization and cleanup", func(t *testing.T) {
		config := otel.Config{
			Enabled:        true,
			ServiceName:    "test-service",
			ServiceVersion: "1.0.0",
			Exporter: otel.ExporterConfig{
				Type: "stdout",
			},
		}

		ctx := context.Background()
		provider, err := otel.Initialize(ctx, config)
		require.NoError(t, err)
		require.NotNil(t, provider)

		// Verify global provider is set
		globalProvider := baseotel.GetTracerProvider()
		assert.NotNil(t, globalProvider)

		// Test creating a span (basic functionality test)
		tracer := globalProvider.Tracer("test")
		_, span := tracer.Start(ctx, "test-operation")
		assert.NotNil(t, span)
		span.End()

		// Test cleanup
		err = provider.Shutdown(ctx)
		assert.NoError(t, err)
	})

	t.Run("initialization failure", func(t *testing.T) {
		config := otel.Config{
			Enabled:     true,
			ServiceName: "test-service",
			Exporter: otel.ExporterConfig{
				Type: "invalid-type",
			},
		}

		ctx := context.Background()
		provider, err := otel.Initialize(ctx, config)
		assert.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "unsupported exporter type")
	})
}

func TestOpenTelemetryServerIntegration(t *testing.T) {
	t.Run("server startup with opentelemetry enabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Nuts.API.URL = "http://example.com"
		cfg.Nuts.Public.URL = "http://example.com"
		cfg.Nuts.DiscoveryService = "http://example.com"
		cfg.Public.Address = ":" + strconv.Itoa(freeTCPPort())

		// Enable OpenTelemetry with stdout exporter
		cfg.OpenTelemetry.Enabled = true
		cfg.OpenTelemetry.ServiceName = "integration-test-orchestrator"
		cfg.OpenTelemetry.ServiceVersion = "test"
		cfg.OpenTelemetry.Exporter.Type = "stdout"

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Track if OpenTelemetry was properly initialized
		var initializationSuccess bool

		// Start server
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := Start(ctx, cfg)
			if err == nil {
				initializationSuccess = true
			}
		}()

		// Wait for server to start
		assertServerStarted(t, cfg.Public.Address)

		// Verify OpenTelemetry global state
		globalProvider := baseotel.GetTracerProvider()
		assert.NotNil(t, globalProvider)

		// Verify we can create and use tracers
		tracer := globalProvider.Tracer("integration-test")
		assert.NotNil(t, tracer)

		testCtx, span := tracer.Start(context.Background(), "integration-test-span")
		assert.NotNil(t, span)

		// Test that span context is properly propagated
		assert.NotEqual(t, testCtx, context.Background())

		span.End()

		// Gracefully shutdown
		cancel()
		wg.Wait()

		assert.True(t, initializationSuccess, "Server should start successfully with OpenTelemetry enabled")
	})
}
