package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
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
	t.Run("results", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
				fhir.Task{
					Id: to.Ptr("456"),
				},
			},
		}
		request := FHIRHandlerRequest{
			ResourcePath:  "Task/_search",
			QueryParams:   map[string][]string{"_id": {"123"}},
			Principal:     auth.TestPrincipal2,
			LocalIdentity: &auth.TestPrincipal1.Organization.Identifier[0],
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
		assert.Len(t, searchResults, 1)
		assert.Contains(t, string(searchResults[0].Resource), "123")
		assert.Empty(t, notifications)
		assert.Len(t, tx.Entry, 1)
		taskRef := fhir.Reference{
			Id:        to.Ptr("123"),
			Type:      to.Ptr("Task"),
			Reference: to.Ptr("Task/123"),
		}
		assertContainsAuditEvent(t, tx.Bundle(), taskRef, auth.TestPrincipal2.Organization.Identifier[0], auth.TestPrincipal1.Organization.Identifier[0], fhir.AuditEventActionR)
	})
	t.Run("no access", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
			},
		}
		request := FHIRHandlerRequest{
			ResourcePath:  "Task/_search",
			QueryParams:   map[string][]string{"_id": {"123"}},
			Principal:     auth.TestPrincipal2,
			LocalIdentity: &auth.TestPrincipal1.Organization.Identifier[0],
		}
		tx := coolfhir.Transaction()
		result, err := FHIRSearchOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: TestPolicy[fhir.Task]{},
		}.Handle(ctx, request, tx)
		assert.NoError(t, err)
		searchResults, notifications, err := result(nil)
		assert.NoError(t, err)
		assert.Empty(t, searchResults)
		assert.Empty(t, notifications)
		assert.Empty(t, tx.Entry)
	})
	t.Run("authz error", func(t *testing.T) {
		fhirClient := &test.StubFHIRClient{
			Resources: []any{
				fhir.Task{
					Id: to.Ptr("123"),
				},
			},
		}
		request := FHIRHandlerRequest{
			ResourcePath:  "Task/_search",
			QueryParams:   map[string][]string{"_id": {"123"}},
			Principal:     auth.TestPrincipal2,
			LocalIdentity: &auth.TestPrincipal1.Organization.Identifier[0],
		}
		tx := coolfhir.Transaction()
		result, err := FHIRSearchOperationHandler[fhir.Task]{
			fhirClient:  fhirClient,
			authzPolicy: TestPolicy[fhir.Task]{Error: assert.AnError},
		}.Handle(ctx, request, tx)
		assert.NoError(t, err)
		searchResults, notifications, err := result(nil)
		assert.NoError(t, err)
		assert.Empty(t, searchResults)
		assert.Empty(t, notifications)
		assert.Empty(t, tx.Entry)
	})
}
