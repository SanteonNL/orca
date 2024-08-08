package healthcheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleHealthCheck(t *testing.T) {
	service := New()
	mux := http.NewServeMux()
	service.RegisterHandlers(mux)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}

	// Create a ResponseRecorder to record the response
	rec := httptest.NewRecorder()

	// Serve the HTTP request
	mux.ServeHTTP(rec, req)

	// Check the status code
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200; got %d", rec.Code)
	}

	// Check the content type
	if rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected content type application/json; got %s", rec.Header().Get("Content-Type"))
	}

	// Check the body
	var response map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("could not decode json response: %v", err)
	}

	expected := map[string]string{"status": "up"}
	if response["status"] != expected["status"] {
		t.Fatalf("expected status up; got %s", response["status"])
	}
}
