package coolfhir

import (
	"log/slog"
	"net/http"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/logging"
)

func Config() *fhirclient.Config {
	config := fhirclient.DefaultConfig()
	config.DefaultOptions = []fhirclient.Option{
		fhirclient.RequestHeaders(map[string][]string{
			"Cache-Control": {"no-cache"},
		}),
	}
	config.Non2xxStatusHandler = func(response *http.Response, responseBody []byte) {
		slog.Debug("Non-2xx status code from FHIR server",
			slog.String("method", response.Request.Method),
			slog.String(logging.FieldUrl, response.Request.URL.String()),
			slog.Int("status", response.StatusCode),
			slog.String("body", string(responseBody)),
		)
	}
	return &config
}
