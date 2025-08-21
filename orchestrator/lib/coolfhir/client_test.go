package coolfhir

import (
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	t.Run("requests specify cache-control: no-cache header", func(t *testing.T) {
		receivedHeaders := make(chan map[string][]string, 1)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedHeaders <- r.Header
			w.Header().Set("Content-Type", "application/fhir+json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"resourceType": "Patient"}`)) // Return an empty Patient resource
		}))
		defer ts.Close()

		client := fhirclient.New(must.ParseURL(ts.URL), http.DefaultClient, Config())

		var target any
		err := client.Read("Patient/example", &target)
		require.NoError(t, err)

		headers := <-receivedHeaders
		require.Equal(t, headers["Cache-Control"], []string{"no-cache"})
	})
}
