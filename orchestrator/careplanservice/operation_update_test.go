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
	"net/http"
	"strings"
	"testing"
)

func TestFHIRUpdateOperationHandler_Handle(t *testing.T) {
	task1ID := "1"
	task1Ref := fhir.Reference{
		Type:      to.Ptr("Task"),
		Reference: to.Ptr("Task/" + task1ID),
	}
	updatedTask := fhir.Task{
		Id:     &task1ID,
		Status: fhir.TaskStatusCancelled,
	}
	existingTask := fhir.Task{
		Id: &task1ID,
	}
	existingTaskWithCreatorExtension := existingTask
	existingTaskWithCreatorExtension.SetExtension(TestCreatorExtension)

	updatedTaskWithCreatorExtension := updatedTask
	updatedTaskWithCreatorExtension.SetExtension(TestCreatorExtension)

	updatedTaskWithArbitraryExtension := updatedTask
	updatedTaskWithArbitraryExtension.SetExtension([]fhir.Extension{
		{
			Url: "http://example.com/extension",
			ValueReference: &fhir.Reference{
				Type: to.Ptr("Organization"),
			},
		},
	})

	toBundle := func(entries []*fhir.BundleEntry) fhir.Bundle {
		result := fhir.Bundle{}
		for _, entry := range entries {
			result.Entry = append(result.Entry, *entry)
		}
		return result
	}

	type args struct {
		resource          fhir.Task
		resourceData      []byte
		existingResources []fhir.Task
	}
	type testCase struct {
		name      string
		args      args
		want      func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult)
		wantErr   assert.ErrorAssertionFunc
		policy    Policy[*fhir.Task]
		fhirError error
	}
	tests := []testCase{
		{
			name: "ok, updated",
			args: args{
				resource:          updatedTaskWithCreatorExtension,
				existingResources: []fhir.Task{existingTaskWithCreatorExtension},
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assertContainsAuditEvent(t, tx, task1Ref, auth.TestPrincipal1.Organization.Identifier[0], auth.TestPrincipal2.Organization.Identifier[0], fhir.AuditEventActionU)
				assertBundleEntry(t, tx, coolfhir.EntryIsOfType(*task1Ref.Type), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, fhir.HTTPVerbPUT, entry.Request.Method)
					assert.Equal(t, *task1Ref.Reference, entry.Request.Url)
					assert.JSONEq(t, string(must.MarshalJSON(updatedTaskWithCreatorExtension)), string(entry.Resource))
				})
				// Should respond with 1 bundle entry, and 1 notification
				txResponse := coolfhir.Transaction().
					Append(updatedTaskWithCreatorExtension, nil, &fhir.BundleEntryResponse{
						Status:   "200 OK",
						Location: task1Ref.Reference,
					}).Bundle()
				responseEntries, notifications, err := result(&txResponse)
				require.NoError(t, err)
				assert.Len(t, responseEntries, 1)
				assertBundleEntry(t, toBundle(responseEntries), coolfhir.EntryIsOfType(*task1Ref.Type), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, "200 OK", entry.Response.Status)
					assert.Equal(t, *task1Ref.Reference, *entry.Response.Location)
					assert.JSONEq(t, string(must.MarshalJSON(updatedTaskWithCreatorExtension)), string(entry.Resource))
				})
				assert.Len(t, notifications, 1)
				assert.IsType(t, &fhir.Task{}, notifications[0])
			},
		},
		{
			name: "ok, upsert",
			args: args{
				resource: updatedTask,
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assertContainsAuditEvent(t, tx, task1Ref, auth.TestPrincipal1.Organization.Identifier[0], auth.TestPrincipal2.Organization.Identifier[0], fhir.AuditEventActionC)
				assertBundleEntry(t, tx, coolfhir.EntryIsOfType(*task1Ref.Type), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, fhir.HTTPVerbPUT, entry.Request.Method)
					assert.Equal(t, *task1Ref.Reference, entry.Request.Url)
					// Validate resource creation extension
					//updatedTask.Extension = TestCreatorExtension
					assert.JSONEq(t, string(must.MarshalJSON(updatedTaskWithCreatorExtension)), string(entry.Resource))
				})
				// Should respond with 1 bundle entry, and 1 notification
				txResponse := coolfhir.Transaction().
					Append(updatedTask, nil, &fhir.BundleEntryResponse{
						Status:   "201 Created",
						Location: task1Ref.Reference,
					}).Bundle()
				responseEntries, notifications, err := result(&txResponse)
				require.NoError(t, err)
				assert.Len(t, responseEntries, 1)
				assertBundleEntry(t, toBundle(responseEntries), coolfhir.EntryIsOfType(*task1Ref.Type), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, "201 Created", entry.Response.Status)
					assert.Equal(t, *task1Ref.Reference, *entry.Response.Location)
					assert.JSONEq(t, string(must.MarshalJSON(updatedTask)), string(entry.Resource))
				})
				assert.Len(t, notifications, 1)
				assert.IsType(t, &fhir.Task{}, notifications[0])
			},
		},
		{
			name: "denied, may not update Extension",
			args: args{
				resource:          updatedTaskWithArbitraryExtension,
				existingResources: []fhir.Task{existingTaskWithCreatorExtension},
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "Task.Extension update not allowed")
			},
		},
		{
			name:   "access denied",
			policy: TestPolicy[*fhir.Task]{},
			args: args{
				resource:          updatedTask,
				existingResources: []fhir.Task{existingTask},
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "Participant is not authorized to update Task")
			},
		},
		{
			name: "access decision failed",
			policy: TestPolicy[*fhir.Task]{
				Error: assert.AnError,
			},
			args: args{
				resource:          updatedTask,
				existingResources: []fhir.Task{existingTask},
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "Participant is not authorized to update Task")
			},
		},
		{
			name:      "existing resource search fails",
			fhirError: assert.AnError,
			args: args{
				resource: updatedTask,
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "failed to search for Task: assert.AnError general error for testing")
			},
		},
		{
			name: "invalid input resource",
			args: args{
				resourceData: []byte("not a valid resource"),
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				expectedErr := new(coolfhir.ErrorWithCode)
				return assert.EqualError(t, err, "invalid Task: invalid character 'o' in literal null (expecting 'u')") &&
					assert.ErrorAs(t, err, &expectedErr) &&
					assert.Equal(t, http.StatusBadRequest, expectedErr.StatusCode)
			},
		},
		{
			name: "invalid external reference",
			args: args{
				resource: fhir.Task{
					Id: &task1ID,
					Requester: &fhir.Reference{
						Type:      to.Ptr("Practitioner"),
						Reference: to.Ptr("http://example.com/Practitioner/123"),
					},
				},
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				expectedErr := new(coolfhir.ErrorWithCode)
				return assert.EqualError(t, err, "literal reference is URL with scheme http://, only https:// is allowed (path=requester.reference)") &&
					assert.ErrorAs(t, err, &expectedErr) &&
					assert.Equal(t, http.StatusBadRequest, expectedErr.StatusCode)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fhirClient := &test.StubFHIRClient{
				Error: tt.fhirError,
			}
			for _, resource := range tt.args.existingResources {
				fhirClient.Resources = append(fhirClient.Resources, resource)
			}
			fhirBaseURL := must.ParseURL("http://example.com/fhir")
			policy := tt.policy
			if policy == nil {
				policy = AnyonePolicy[*fhir.Task]{}
			}
			handler := &FHIRUpdateOperationHandler[*fhir.Task]{
				authzPolicy: policy,
				fhirClient:  fhirClient,
				profile:     profile.Test(),
				fhirURL:     fhirBaseURL,
				createHandler: &FHIRCreateOperationHandler[*fhir.Task]{
					authzPolicy: policy,
					fhirClient:  fhirClient,
					profile:     profile.Test(),
					fhirURL:     fhirBaseURL,
				},
			}
			requestData := tt.args.resourceData
			if requestData == nil {
				requestData = must.MarshalJSON(tt.args.resource)
			}
			tx := coolfhir.Transaction()
			handlerResult, err := handler.Handle(context.Background(), FHIRHandlerRequest{
				HttpMethod:    http.MethodPut,
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
			tt.want(t, tx.Bundle(), handlerResult)
		})
	}
}

func assertBundleEntry(t *testing.T, tx fhir.Bundle, filter func(entry fhir.BundleEntry) bool, asserter func(t *testing.T, entry fhir.BundleEntry)) {
	entry := coolfhir.FirstBundleEntry(&tx, filter)
	require.NotNil(t, entry)
	asserter(t, *entry)
}

func assertContainsAuditEvent(t *testing.T, tx fhir.Bundle, what fhir.Reference, actor fhir.Identifier, observer fhir.Identifier, action fhir.AuditEventAction) {
	for _, entry := range tx.Entry {
		if entry.Request.Url != "AuditEvent" {
			continue
		}
		assert.Equal(t, fhir.HTTPVerbPOST, entry.Request.Method)
		var evt fhir.AuditEvent
		err := json.Unmarshal(entry.Resource, &evt)
		require.NoError(t, err)
		assert.Equal(t, action.Code(), evt.Action.Code())
		assert.Equal(t, actor, *evt.Agent[0].Who.Identifier)
		assert.Equal(t, fhir.Reference{
			Type:       to.Ptr("Device"),
			Identifier: &observer,
		}, evt.Source.Observer)
		assert.Len(t, evt.Entity, 1)
		assert.Equal(t, *what.Type, *evt.Entity[0].What.Type)
		if strings.HasPrefix(*evt.Entity[0].What.Reference, "urn:uuid:") {
			// Local references are to be resolved by the FHIR server, so we can't check them. But, we can check the type.
			assert.Equal(t, *what.Type, *evt.Entity[0].What.Type)
		} else {
			assert.Equal(t, *what.Reference, *evt.Entity[0].What.Reference)
		}
		return
	}
	assert.Fail(t, "AuditEvent not found in transaction bundle")
}
