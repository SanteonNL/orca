package careplanservice

import (
	"context"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"testing"
)

func TestFHIRReadOperationHandler_Handle(t *testing.T) {
	ctx := context.Background()
	t.Run("not found", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{}
		request := FHIRHandlerRequest{
			ResourcePath: "Task/1",
			ResourceId:   "1",
		}
		tx := coolfhir.Transaction()
		result, err := FHIRReadOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: AnyonePolicy[fhir.Task]{},
		}.Handle(ctx, request, tx)
		assert.Error(t, err)
		var outcome fhirclient.OperationOutcomeError
		require.ErrorAs(t, err, &outcome)
		assert.Equal(t, http.StatusNotFound, outcome.HttpStatusCode)
		assert.Nil(t, result)
		assert.Empty(t, tx.Entry)
	})
	t.Run("no access", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("1"),
				},
			},
		}
		request := FHIRHandlerRequest{
			ResourcePath: "Task/1",
			ResourceId:   "1",
			Principal:    auth.TestPrincipal1,
		}
		tx := coolfhir.Transaction()
		result, err := FHIRReadOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: TestPolicy[fhir.Task]{},
		}.Handle(ctx, request, tx)
		assert.Error(t, err)
		errorWithCode := new(coolfhir.ErrorWithCode)
		require.ErrorAs(t, err, &errorWithCode)
		assert.Equal(t, http.StatusForbidden, errorWithCode.StatusCode)
		assert.Equal(t, "Participant does not have access to Task", errorWithCode.Message)
		assert.Nil(t, result)
		assert.Empty(t, tx.Entry)
	})
	t.Run("ok", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("1"),
				},
			},
		}
		request := FHIRHandlerRequest{
			ResourcePath: "Task/1",
			ResourceId:   "1",
			Principal:    auth.TestPrincipal1,
			LocalIdentity: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("1"),
			},
		}
		tx := coolfhir.Transaction()
		result, err := FHIRReadOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: AnyonePolicy[fhir.Task]{},
		}.Handle(ctx, request, tx)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

type TestPolicy[T any] struct {
	Allow bool
	Error error
}

func (n TestPolicy[T]) HasAccess(ctx context.Context, resource T, principal auth.Principal) (bool, error) {
	return n.Allow, n.Error
}
