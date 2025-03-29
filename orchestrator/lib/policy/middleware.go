package policy

import (
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
)

type ContextExtractor func(r *http.Request) (any, error)

func (agent Agent) WrapWithPolicyCheck(extractContext ContextExtractor, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input, err := extractContext(r)
		if err != nil {
			agent.logger.Err(err).Msg("failed to extract context")
			fhirError := coolfhir.NewErrorWithCode("policy error: error extracting context", http.StatusInternalServerError)
			coolfhir.WriteOperationOutcomeFromError(r.Context(), fhirError, "error extracting context", w)
			return
		}

		if err := agent.Allow(r.Context(), input, r); err != nil {
			agent.logger.Debug().Err(err).Msg("request not allowed")
			fhirError := coolfhir.NewErrorWithCode("request not allowed due to policy", http.StatusForbidden)
			coolfhir.WriteOperationOutcomeFromError(r.Context(), fhirError, "request not allowed", w)
			return
		}

		handler(w, r)
	}
}
