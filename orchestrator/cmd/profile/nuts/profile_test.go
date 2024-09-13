package nuts

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDutchNutsProfile_RegisterHTTPHandlers(t *testing.T) {
	const basePath = "/basement"
	var baseURL, _ = url.Parse("http://example.com" + basePath)
	serverMux := http.NewServeMux()
	DutchNutsProfile{}.RegisterHTTPHandlers("/basement", baseURL, serverMux)
	server := httptest.NewServer(serverMux)

	httpResponse, err := http.Get(server.URL + "/basement/.well-known/oauth-protected-resource")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	data, _ := io.ReadAll(httpResponse.Body)
	assert.JSONEq(t, `{"resource":"http://example.com/basement","authorization_servers":["oauth2"],"bearer_methods_supported":["header"]}`, string(data))
}
