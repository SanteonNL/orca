package cmd

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/cmd/tenants"
	"github.com/SanteonNL/orca/orchestrator/globals"
	"github.com/stretchr/testify/require"
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
		properties.ChipSoftOrgID = ""
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
