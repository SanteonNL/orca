package careplancontributor

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type fhirWriter struct {
	StatusCode int
	Headers    http.Header
	Resource   coolfhir.RawResource
}

func newFhirWriter() *fhirWriter {
	return &fhirWriter{Headers: http.Header{}}
}

func (w *fhirWriter) Header() http.Header {
	return w.Headers
}

func (w *fhirWriter) Write(b []byte) (int, error) {
	if err := json.Unmarshal(b, &w.Resource); err != nil {
		return -1, fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	return len(b), nil
}

func (w *fhirWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}

func (w *fhirWriter) Bundle() (*fhir.Bundle, error) {
	if w.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status: %d", w.StatusCode)
	}

	if w.Resource.Type != "Bundle" {
		return nil, fmt.Errorf("invalid resource type: %s", w.Resource.Type)
	}

	var bundle fhir.Bundle

	if err := json.Unmarshal(w.Resource.Raw, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bundle: %w", err)
	}

	return &bundle, nil
}
