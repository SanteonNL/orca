package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	events "github.com/SanteonNL/orca/orchestrator/events"
)

// See: https://go.googlesource.com/go/+/refs/tags/go1.20.4/src/net/http/server.go#1098
const maxPostHandlerReadBytes = 256 << 10

var _ events.Handler = EventHandler{}

func disposeResponseBody(r io.ReadCloser) {
	if _, err := io.CopyN(io.Discard, r, maxPostHandlerReadBytes); err != nil {
		slog.Error("failed to read response body", slog.String("error", err.Error()))
	}
}

type EventHandler struct {
	client *http.Client
	URL    string
}

func NewEventHandler(url string) EventHandler {
	return EventHandler{URL: url, client: &http.Client{}}
}

func (w EventHandler) Handle(ctx context.Context, event events.Type) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Add("Content-Type", "application/fhir+json")

	response, err := w.client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	defer disposeResponseBody(response.Body)

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	slog.InfoContext(ctx, "Successfully sent event to webhook", slog.String("url", w.URL))

	return nil
}
