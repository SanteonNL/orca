package keys

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSigningKeyFromAzureKeyVault(t *testing.T) {
	mux := http.NewServeMux()
	httpServer := httptest.NewTLSServer(mux)
	defer httpServer.Close()
	mux.HandleFunc("GET /keys/keyz/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := os.ReadFile("azure-getkey-response.json")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	AzureHttpRequestDoer = httpServer.Client()
	signingKey, err := signingKeyFromAzureKeyVault(httpServer.URL, "keyz", true)
	require.NoError(t, err)

	assert.Equal(t, "https://keyszzz.vault.azure.net/keys/signingkey/5072fbaaa30849298e4b3c60384cdaac", signingKey.KeyID())
	assert.Equal(t, "ES256", signingKey.SigningAlgorithm())
}
