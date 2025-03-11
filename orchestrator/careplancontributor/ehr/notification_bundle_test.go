package ehr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"testing"
)

func TestTaskNotificationBundleSet(t *testing.T) {
	t.Skip("These tests are refactored out in another change and are not relevant anymore")
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	patient1Raw, _ := json.Marshal(fhir.Patient{Id: to.Ptr("1")})
	carePlan1Raw, _ := json.Marshal(fhir.CarePlan{
		Id: to.Ptr("1"),
		Subject: fhir.Reference{
			Reference: to.Ptr("Patient/1"),
		},
	})
	careTeam1Raw, _ := json.Marshal(fhir.CareTeam{Id: to.Ptr("1")})
	serviceRequest1Raw, _ := json.Marshal(fhir.ServiceRequest{Id: to.Ptr("1")})
	questionnaire1Raw, _ := json.Marshal(fhir.Questionnaire{Id: to.Ptr("1")})
	questionnaireResponse1Raw, _ := json.Marshal(fhir.QuestionnaireResponse{Id: to.Ptr("1")})

	task := fhir.Task{
		Id: to.Ptr("1"),
		For: &fhir.Reference{
			Reference: to.Ptr("Patient/1"),
		},
		BasedOn: []fhir.Reference{
			{Reference: to.Ptr("CarePlan/1")},
		},
		Focus: &fhir.Reference{
			Reference: to.Ptr("ServiceRequest/1"),
		},
		Input: []fhir.TaskInput{
			{
				ValueReference: &fhir.Reference{
					Reference: to.Ptr("Questionnaire/1"),
				},
			},
		},
		Output: []fhir.TaskOutput{
			{
				ValueReference: &fhir.Reference{
					Reference: to.Ptr("QuestionnaireResponse/1"),
				},
			},
		},
	}
	task1Raw, _ := json.Marshal(task)

	subtask := fhir.Task{
		Id: to.Ptr("2"),
		PartOf: []fhir.Reference{
			{Reference: to.Ptr("Task/1")},
		},
	}
	subtask1Raw, _ := json.Marshal(subtask)

	t.Run("return bundle set - success", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: patient1Raw},
				},
			}
			return nil
		})
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: carePlan1Raw},
				},
			}
			return nil
		})
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: serviceRequest1Raw},
				},
			}
			return nil
		})
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Questionnaire", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: questionnaire1Raw},
				},
			}
			return nil
		})
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "QuestionnaireResponse", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: questionnaireResponse1Raw},
				},
			}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "Task/1", result.task)
		require.Equal(t, 6, len(result.Bundles))

		// Check if the bundles are in the correct order
		require.JSONEq(t, string(task1Raw), string(result.Bundles[0].Entry[0].Resource))
		require.JSONEq(t, string(subtask1Raw), string(result.Bundles[0].Entry[1].Resource))
		require.JSONEq(t, string(patient1Raw), string(result.Bundles[1].Entry[0].Resource))
		require.JSONEq(t, string(serviceRequest1Raw), string(result.Bundles[2].Entry[0].Resource))
		require.JSONEq(t, string(carePlan1Raw), string(result.Bundles[3].Entry[0].Resource))
		require.JSONEq(t, string(careTeam1Raw), string(result.Bundles[3].Entry[1].Resource))
		require.JSONEq(t, string(questionnaire1Raw), string(result.Bundles[4].Entry[0].Resource))
		require.JSONEq(t, string(questionnaireResponse1Raw), string(result.Bundles[5].Entry[0].Resource))
	})
	t.Run("error fetching task - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).Return(errors.New("error"))

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("error fetching carePlan - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).Return(errors.New("error"))

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("search does not find resource - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: carePlan1Raw},
					{Resource: careTeam1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			// Simulating a search that does not find any results, this does not return an error
			*data = fhir.Bundle{}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("no subtask - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
				},
			}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("no patient - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: carePlan1Raw},
					{Resource: careTeam1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("no service request - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: carePlan1Raw},
					{Resource: careTeam1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: patient1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "ServiceRequest", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
	t.Run("no careplan - fails", func(t *testing.T) {
		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Task", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{Resource: task1Raw},
					{Resource: subtask1Raw},
				},
			}
			return nil
		})

		mockFHIRClient.EXPECT().SearchWithContext(ctx, "CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = fhir.Bundle{
				Entry: []fhir.BundleEntry{},
			}
			return nil
		})

		result, err := TaskNotificationBundleSet(ctx, mockFHIRClient, "1")
		require.Error(t, err)
		require.Nil(t, result)
	})
}

func TestFetchRefs(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	patient1Raw, _ := json.Marshal(fhir.Patient{Id: to.Ptr("1")})
	patient2Raw, _ := json.Marshal(fhir.Patient{Id: to.Ptr("2")})

	t.Run("successful fetch", func(t *testing.T) {
		refs := []string{"Patient/1", "Patient/2"}
		expectedBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{Resource: patient1Raw},
				{Resource: patient2Raw},
			},
		}

		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = expectedBundle
			return nil
		})

		result, err := fetchRefs(ctx, mockFHIRClient, refs)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 1, len(*result))
		require.Equal(t, expectedBundle, (*result)[0])
	})

	t.Run("fetch error", func(t *testing.T) {
		refs := []string{"Patient/1"}

		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			return errors.New("error")
		})

		result, err := fetchRefs(ctx, mockFHIRClient, refs)
		require.Error(t, err)
		require.Nil(t, result)
	})

	t.Run("missing entries", func(t *testing.T) {
		refs := []string{"Patient/1", "Patient/2"}
		expectedBundle := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{Resource: patient1Raw},
			},
		}

		mockFHIRClient := mock.NewMockClient(ctrl)
		mockFHIRClient.EXPECT().SearchWithContext(ctx, "Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, resource string, searchParams url.Values, data *fhir.Bundle, opts ...interface{}) error {
			*data = expectedBundle
			return nil
		})

		result, err := fetchRefs(ctx, mockFHIRClient, refs)
		require.Error(t, err)
		require.Nil(t, result)
		require.EqualError(t, err, fmt.Sprintf("failed to fetch all references of type %s, expected %d bundle entries, got %d", "Patient", len(refs), len(expectedBundle.Entry)))
	})
}

func TestIsOfType(t *testing.T) {
	type1 := "Questionnaire"
	type2 := "QuestionnaireResponse"
	type3 := "https://example.com/Questionnaire/123"
	type4 := "https://example.com/QuestionnaireResponse/123"
	type6 := "https://example.com/Questionnaire"
	type5 := "Questionnaire/123"
	tests := []struct {
		name           string
		valueReference *fhir.Reference
		typeName       string
		expected       bool
	}{
		{
			name: "type matches directly",
			valueReference: &fhir.Reference{
				Type: &type1,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "type does not match directly",
			valueReference: &fhir.Reference{
				Type: &type2,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "reference matches with https prefix",
			valueReference: &fhir.Reference{
				Reference: &type3,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does not match with https prefix",
			valueReference: &fhir.Reference{
				Reference: &type4,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "reference matches without https prefix",
			valueReference: &fhir.Reference{
				Reference: &type5,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does match without https prefix",
			valueReference: &fhir.Reference{
				Reference: &type5,
			},
			typeName: "Questionnaire",
			expected: true,
		},
		{
			name: "reference does match without value",
			valueReference: &fhir.Reference{
				Reference: &type6,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "nil reference",
			valueReference: &fhir.Reference{
				Reference: nil,
			},
			typeName: "Questionnaire",
			expected: false,
		},
		{
			name: "trigger a compilation error",
			valueReference: &fhir.Reference{
				Reference: &type4,
			},
			typeName: "(",
			expected: false,
		},
		{
			name: "nil type and reference",
			valueReference: &fhir.Reference{
				Type:      nil,
				Reference: nil,
			},
			typeName: "Questionnaire",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isOfType(tt.valueReference, tt.typeName)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestPutMapListValue(t *testing.T) {
	type args struct {
		refTypeMap map[string][]string
		refType    string
		refId      string
	}
	tests := []struct {
		name        string
		args        args
		expectedMap map[string][]string
	}{
		{
			name: "add new value",
			args: args{
				refTypeMap: map[string][]string{},
				refType:    "type",
				refId:      "id",
			},
			expectedMap: map[string][]string{
				"type": {"id"},
			},
		},
		{
			name: "add existing value",
			args: args{
				refTypeMap: map[string][]string{
					"type": {"id"},
				},
				refType: "type",
				refId:   "id",
			},
			expectedMap: map[string][]string{
				"type": {"id"},
			},
		},
		{
			name: "add new value to existing type",
			args: args{
				refTypeMap: map[string][]string{
					"type": {"id"},
				},
				refType: "type",
				refId:   "id2",
			},
			expectedMap: map[string][]string{
				"type": {"id", "id2"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			putMapListValue(tt.args.refTypeMap, tt.args.refType, tt.args.refId)

			require.Equal(t, tt.expectedMap, tt.args.refTypeMap)
		})
	}
}
