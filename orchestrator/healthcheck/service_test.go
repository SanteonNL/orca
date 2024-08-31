package healthcheck

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealthCheck(t *testing.T) {
	service := New()
	mux := http.NewServeMux()
	service.RegisterHandlers(mux)
	req, _ := http.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	// Check the body
	var response map[string]string
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)
	expected := map[string]string{"status": "up"}
	require.Equal(t, expected, response)
}
