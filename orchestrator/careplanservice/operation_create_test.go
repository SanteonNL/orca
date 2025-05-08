package careplanservice

import (
	"context"
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
	"testing"
)

func TestFHIRCreateOperationHandler_Handle(t *testing.T) {
	task := fhir.Task{
		Status: fhir.TaskStatusCancelled,
	}
	toBundle := func(entries []*fhir.BundleEntry) fhir.Bundle {
		result := fhir.Bundle{}
		for _, entry := range entries {
			result.Entry = append(result.Entry, *entry)
		}
		return result
	}

	type args struct {
		resource     fhir.Task
		resourceData []byte
	}
	type testCase struct {
		name      string
		args      args
		want      func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult)
		wantErr   assert.ErrorAssertionFunc
		policy    Policy[fhir.Task]
		fhirError error
	}
	tests := []testCase{
		{
			name: "ok",
			args: args{
				resource: task,
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assertContainsAuditEvent(t, tx, fhir.Reference{Type: to.Ptr("Task")}, auth.TestPrincipal1.Organization.Identifier[0], auth.TestPrincipal2.Organization.Identifier[0], fhir.AuditEventActionC)
				assertBundleEntry(t, tx, coolfhir.EntryIsOfType("Task"), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, fhir.HTTPVerbPOST, entry.Request.Method)
					assert.Equal(t, "Task", entry.Request.Url)
					assert.JSONEq(t, string(must.MarshalJSON(task)), string(entry.Resource))
				})
				// Should respond with 1 bundle entry, and 1 notification
				txResponse := coolfhir.Transaction().
					Append(task, nil, &fhir.BundleEntryResponse{
						Status:   "200 OK",
						Location: to.Ptr("Task/1"),
					}).Bundle()
				responseEntries, notifications, err := result(&txResponse)
				require.NoError(t, err)
				assert.Len(t, responseEntries, 1)
				assertBundleEntry(t, toBundle(responseEntries), coolfhir.EntryIsOfType("Task"), func(t *testing.T, entry fhir.BundleEntry) {
					assert.Equal(t, "200 OK", entry.Response.Status)
					assert.Equal(t, "Task/1", *entry.Response.Location)
					assert.JSONEq(t, string(must.MarshalJSON(task)), string(entry.Resource))
				})
				assert.Len(t, notifications, 1)
				assert.IsType(t, &fhir.Task{}, notifications[0])
			},
		},
		{
			name:   "access denied",
			policy: TestPolicy[fhir.Task]{},
			args: args{
				resource: task,
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "Participant is not authorized to create Task")
			},
		},
		{
			name: "access decision failed",
			policy: TestPolicy[fhir.Task]{
				Error: assert.AnError,
			},
			args: args{
				resource: task,
			},
			want: func(t *testing.T, tx fhir.Bundle, result FHIRHandlerResult) {
				assert.Empty(t, tx.Entry)
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.EqualError(t, err, "Participant is not authorized to create Task")
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
			fhirClient := &test.StubFHIRClient{}
			fhirBaseURL := must.ParseURL("http://example.com/fhir")
			policy := tt.policy
			if policy == nil {
				policy = AnyonePolicy[fhir.Task]{}
			}
			handler := &FHIRCreateOperationHandler[fhir.Task]{
				authzPolicy: policy,
				fhirClient:  fhirClient,
				profile:     profile.Test(),
				fhirURL:     fhirBaseURL,
			}
			requestData := tt.args.resourceData
			if requestData == nil {
				requestData = must.MarshalJSON(tt.args.resource)
			}
			tx := coolfhir.Transaction()
			handlerResult, err := handler.Handle(context.Background(), FHIRHandlerRequest{
				HttpMethod:    http.MethodPost,
				ResourceData:  requestData,
				ResourcePath:  "Task",
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
