package careplanservice

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestFHIRUpdateOperationHandler_Handle(t *testing.T) {
	task1ID := "1"
	task1Ref := fhir.Reference{
		Type:      to.Ptr("Task"),
		Reference: to.Ptr("Task/" + task1ID),
	}

	type args struct {
		resource          fhir.Task
		existingResources []fhir.Task
	}
	type testCase struct {
		name    string
		args    args
		want    func(t *testing.T, tx fhir.Bundle)
		wantErr assert.ErrorAssertionFunc
	}
	tests := []testCase{
		{
			name: "ok, updated",
			args: args{
				resource: fhir.Task{
					Id: &task1ID,
				},
				existingResources: []fhir.Task{
					{
						Id: &task1ID,
					},
				},
			},
			want: func(t *testing.T, tx fhir.Bundle) {
				assertContainsAuditEvent(t, tx, task1Ref, auth.TestPrincipal1.Organization.Identifier[0], auth.TestPrincipal2.Organization.Identifier[0])
			},
		},
		{
			name: "ok, upsert",
			args: args{
				resource: fhir.Task{
					Id: to.Ptr("1"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fhirClient := &test.StubFHIRClient{}
			for _, resource := range tt.args.existingResources {
				fhirClient.Resources = append(fhirClient.Resources, resource)
			}
			fhirBaseURL := must.ParseURL("http://example.com/fhir")
			handler := &FHIRUpdateOperationHandler[fhir.Task]{
				authzPolicy: AnyonePolicy[fhir.Task]{},
				fhirClient:  fhirClient,
				profile:     profile.Test(),
				fhirURL:     fhirBaseURL,
				createHandler: &FHIRCreateOperationHandler[fhir.Task]{
					authzPolicy: AnyonePolicy[fhir.Task]{},
					fhirClient:  fhirClient,
					profile:     profile.Test(),
					fhirURL:     fhirBaseURL,
				},
			}
			requestData, _ := json.Marshal(tt.args.resource)
			tx := coolfhir.Transaction()
			handlerResult, err := handler.Handle(context.Background(), FHIRHandlerRequest{
				ResourceId:    "1",
				ResourceData:  requestData,
				ResourcePath:  "Task/1",
				Principal:     auth.TestPrincipal1,
				LocalIdentity: &auth.TestPrincipal2.Organization.Identifier[0],
			}, tx)
			if tt.wantErr != nil {
				tt.wantErr(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, handlerResult)
			}
			if tt.want != nil {
				tt.want(t, tx.Bundle())
			}
		})
	}
}

func assertContainsAuditEvent(t *testing.T, tx fhir.Bundle, what fhir.Reference, actor fhir.Identifier, observer fhir.Identifier) {
	for _, entry := range tx.Entry {
		if entry.Request.Url != "AuditEvent" {
			continue
		}
		assert.Equal(t, fhir.HTTPVerbPOST, entry.Request.Method)
		var evt fhir.AuditEvent
		err := json.Unmarshal(entry.Resource, &evt)
		require.NoError(t, err)
		assert.Equal(t, fhir.AuditEventActionU.Code(), evt.Action.Code())
		assert.Equal(t, actor, *evt.Agent[0].Who.Identifier)
		assert.Equal(t, observer, evt.Source.Observer)
		assert.Len(t, evt.Entity, 1)
		assert.Equal(t, *what.Type, evt.Entity[0].What.Type)
		assert.Equal(t, *what.Reference, *evt.Entity[0].What.Reference)
		return
	}
	assert.Fail(t, "AuditEvent not found in transaction bundle")
}
