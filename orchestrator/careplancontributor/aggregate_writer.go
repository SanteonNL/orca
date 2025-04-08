package careplancontributor

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

type aggregateWriter struct {
	StatusCode int
	Entries    chan fhir.BundleEntry
}

func (w aggregateWriter) Header() http.Header {
	return http.Header{}
}

func (w aggregateWriter) Write(b []byte) (int, error) {
	var resource coolfhir.RawResource

	if err := json.Unmarshal(b, &resource); err != nil {
		return -1, fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	if resource.Type == "Bundle" {
		var bundle fhir.Bundle

		if err := json.Unmarshal(resource.Raw, &bundle); err != nil {
			return -1, fmt.Errorf("failed to unmarshal bundle: %w", err)
		}

		for _, entry := range bundle.Entry {
			w.Entries <- entry
		}
	} else {
		w.Entries <- fhir.BundleEntry{
			Id:       &resource.ID,
			Resource: resource.Raw,
		}
	}

	return len(b), nil
}

func (w aggregateWriter) WriteHeader(statusCode int) {
	w.StatusCode = statusCode
}
