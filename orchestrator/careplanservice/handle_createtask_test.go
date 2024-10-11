package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	fhirclient_mock "github.com/SanteonNL/orca/orchestrator/mocks/github.com/SanteonNL/go-fhir-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_basedOn(t *testing.T) {
	type args struct {
		task fhir.Task
	}
	tests := []struct {
		name    string
		args    args
		want    *string
		wantErr error
	}{
		{
			name: "basedOn references a CarePlan (OK)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("CarePlan/123"),
						},
					},
				},
			},
			want:    to.Ptr("CarePlan/123"),
			wantErr: nil,
		},
		{
			name: "no basedOn",
			args: args{
				task: fhir.Task{},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn contains multiple references (instead of 1)",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{},
						{},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must have exactly one reference"),
		},
		{
			name: "basedOn does not reference a CarePlan",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Reference: to.Ptr("Patient/2"),
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
		{
			name: "basedOn is not a relative reference",
			args: args{
				task: fhir.Task{
					BasedOn: []fhir.Reference{
						{
							Type:       to.Ptr("CarePlan"),
							Identifier: &fhir.Identifier{},
						},
					},
				},
			},
			want:    nil,
			wantErr: errors.New("Task.basedOn must contain a relative reference to a CarePlan"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := basedOn(tt.args.task)
			if tt.wantErr != nil {
				require.EqualError(t, gotErr, tt.wantErr.Error())
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_handleCreateTask(t *testing.T) {
	t.Run("create Task - no existing CarePlan", func(t *testing.T) {
		mockClient := fhirclient_mock.NewMockClient(t)
		service := Service{
			fhirClient:      mockClient,
			maxReadBodySize: 1024 * 1024,
		}
		createdTask := fhir.Task{
			BasedOn: []fhir.Reference{
				{
					Type:      to.Ptr("CarePlan"),
					Reference: to.Ptr("CarePlan/123"),
				},
			},
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}
		txResult := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CarePlan/123"),
						Status:   "204 Created",
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CareTeam/123"),
						Status:   "204 Created",
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("Task/123"),
						Status:   "204 Created",
					},
				},
			},
		}
		mockClient.EXPECT().Read("Task/123", mock.Anything, mock.Anything).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
			data, _ := json.Marshal(createdTask)
			*(result.(*[]byte)) = data
			return nil
		})
		task := fhir.Task{
			Intent:    "order",
			Status:    fhir.TaskStatusRequested,
			Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
			Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
		}
		taskBytes, _ := json.Marshal(task)
		fhirRequest := FHIRHandlerRequest{
			ResourcePath: "Task",
			ResourceData: taskBytes,
			HttpMethod:   "POST",
		}

		tx := coolfhir.Transaction()
		result, err := service.handleCreateTask(auth.WithPrincipal(context.Background(), *auth.TestPrincipal1), fhirRequest, tx)

		require.NoError(t, err)

		// Assert it creates a CareTeam, CarePlan, and a Task
		require.Len(t, tx.Entry, 3)
		assert.Equal(t, "CarePlan", tx.Entry[0].Request.Url)
		assert.Equal(t, "CareTeam", tx.Entry[1].Request.Url)
		assert.Equal(t, "Task", tx.Entry[2].Request.Url)

		// Process result
		require.NotNil(t, result)
		response, notifications, err := result(&txResult)
		require.NoError(t, err)
		assert.Len(t, notifications, 1)
		require.Equal(t, "Task/123", *response.Response.Location)
		require.Equal(t, "204 Created", response.Response.Status)
	})
	// TODO: Get this working
	//t.Run("create Task - existing CarePlan", func(t *testing.T) {
	//	mockClient := fhirclient_mock.NewMockClient(t)
	//	service := Service{
	//		fhirClient:      mockClient,
	//		maxReadBodySize: 1024 * 1024,
	//	}
	//	existingCarePlan := fhir.CarePlan{
	//		CareTeam: []fhir.Reference{
	//			{
	//				Reference: to.Ptr("CareTeam/123"),
	//			},
	//		},
	//	}
	//	existingCareTeam := fhir.CareTeam{
	//		Participant: []fhir.CareTeamParticipant{
	//			{
	//				OnBehalfOf: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
	//				Period:     &fhir.Period{Start: to.Ptr("2021-01-01T00:00:00Z")},
	//			},
	//		},
	//	}
	//	createdTask := fhir.Task{
	//		BasedOn: []fhir.Reference{
	//			{
	//				Type:      to.Ptr("CarePlan"),
	//				Reference: to.Ptr("CarePlan/123"),
	//			},
	//		},
	//		Intent:    "order",
	//		Status:    fhir.TaskStatusRequested,
	//		Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
	//		Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
	//	}
	//	txResult := fhir.Bundle{
	//		Entry: []fhir.BundleEntry{
	//			{
	//				Response: &fhir.BundleEntryResponse{
	//					Location: to.Ptr("CarePlan/123"),
	//					Status:   "204 Created",
	//				},
	//			},
	//			{
	//				Response: &fhir.BundleEntryResponse{
	//					Location: to.Ptr("CareTeam/123"),
	//					Status:   "204 Created",
	//				},
	//			},
	//			{
	//				Response: &fhir.BundleEntryResponse{
	//					Location: to.Ptr("Task/123"),
	//					Status:   "204 Created",
	//				},
	//			},
	//		},
	//	}
	//	//mockClient.EXPECT().Read("CarePlan/123", mock.Anything, fhirclient.ResolveRef("careTeam", &referencedResource)).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
	//	//	data, _ := json.Marshal(existingCarePlan)
	//	//	*(result.(*[]byte)) = data
	//	//	referencedResource = []fhir.CareTeam{existingCareTeam}
	//	//	return nil
	//	//})
	//	mockClient.EXPECT().Read("CareTeam/123", mock.Anything).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
	//		data, _ := json.Marshal(existingCareTeam)
	//		*(result.(*[]byte)) = data
	//		return nil
	//	})
	//	mockClient.EXPECT().Read("CarePlan/123", mock.Anything, mock.AnythingOfType("fhirclient.PostParseOption")).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
	//		data, _ := json.Marshal(existingCarePlan)
	//		*(result.(*[]byte)) = data
	//		return nil
	//	})
	//	mockClient.EXPECT().Read("Task/123", mock.Anything, mock.Anything).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
	//		data, _ := json.Marshal(createdTask)
	//		*(result.(*[]byte)) = data
	//		return nil
	//	})
	//	//mockClient.EXPECT().Read("CareTeam/123", mock.Anything, mock.Anything).RunAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
	//	//	data, _ := json.Marshal(existingCareTeam)
	//	//	*(result.(*[]byte)) = data
	//	//	return nil
	//	//})
	//	task := fhir.Task{
	//		BasedOn: []fhir.Reference{
	//			{
	//				Type:      to.Ptr("CarePlan"),
	//				Reference: to.Ptr("CarePlan/123"),
	//			},
	//		},
	//		Intent:    "order",
	//		Status:    fhir.TaskStatusRequested,
	//		Requester: coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "1"),
	//		Owner:     coolfhir.LogicalReference("Organization", coolfhir.URANamingSystem, "2"),
	//	}
	//	taskBytes, _ := json.Marshal(task)
	//	fhirRequest := FHIRHandlerRequest{
	//		ResourcePath: "Task",
	//		ResourceData: taskBytes,
	//		HttpMethod:   "POST",
	//	}
	//
	//	tx := coolfhir.Transaction()
	//	result, err := service.handleCreateTask(auth.WithPrincipal(context.Background(), *auth.TestPrincipal1), fhirRequest, tx)
	//
	//	require.NoError(t, err)
	//
	//	// Assert it creates a CareTeam, CarePlan, and a Task
	//	require.Len(t, tx.Entry, 3)
	//	assert.Equal(t, "CarePlan", tx.Entry[0].Request.Url)
	//	assert.Equal(t, "CareTeam", tx.Entry[1].Request.Url)
	//	assert.Equal(t, "Task", tx.Entry[2].Request.Url)
	//
	//	// Process result
	//	require.NotNil(t, result)
	//	response, notifications, err := result(&txResult)
	//	require.NoError(t, err)
	//	assert.Len(t, notifications, 1)
	//	require.Equal(t, "Task/123", *response.Response.Location)
	//	require.Equal(t, "204 Created", response.Response.Status)
	//})
}
