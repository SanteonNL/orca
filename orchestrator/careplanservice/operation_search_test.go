package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestFHIRSearchOperationHandler_Handle(t *testing.T) {
	ctx := context.Background()
	t.Run("no results", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{}
		request := FHIRHandlerRequest{
			ResourcePath: "Task/_search",
			QueryParams:  map[string][]string{},
		}
		tx := coolfhir.Transaction()
		result, err := FHIRSearchOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: AnyonePolicy[fhir.Task]{},
		}.Handle(ctx, request, tx)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		searchResults, notifications, err := result(nil)
		assert.NoError(t, err)
		assert.Empty(t, searchResults)
		assert.Empty(t, notifications)
	})
}
