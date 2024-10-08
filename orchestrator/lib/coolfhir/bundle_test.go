package coolfhir

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"io"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func TestTransactionBuilder(t *testing.T) {
	tx := Transaction().
		Create(fhir.Task{
			Id: to.Ptr("task1"),
		}).
		Create(fhir.Task{
			Id: to.Ptr("task2"),
		}).
		Bundle()

	require.Equal(t, fhir.BundleTypeTransaction, tx.Type)
	require.Len(t, tx.Entry, 2)

	var task1 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[0].Resource, &task1))
	require.Equal(t, "task1", task1["id"])
	var task2 map[string]interface{}
	require.NoError(t, json.Unmarshal(tx.Entry[1].Resource, &task2))
	require.Equal(t, "task2", task2["id"])
}

func TestExecuteTransactionAndRespondWithEntry(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		tx := Transaction().Create(fhir.CarePlan{Id: to.Ptr("careplan1")})
		fhirBundleResult := fhir.Bundle{
			Entry: []fhir.BundleEntry{
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("CarePlan/careplan1"),
					},
				},
				{
					Response: &fhir.BundleEntryResponse{
						Location: to.Ptr("Task/task1"),
					},
				},
			},
		}
		fhirCreatedTask := fhir.Task{
			Id:     to.Ptr("task1"),
			Intent: "order",
			Status: fhir.TaskStatusCompleted,
		}

		t.Run("provide result struct", func(t *testing.T) {
			fhirClient := mock.NewMockClient(ctrl)
			fhirClient.EXPECT().Create(tx.Bundle(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ fhir.Bundle, result *fhir.Bundle, _ interface{}) error {
					*result = fhirBundleResult
					return nil
				})
			fhirClient.EXPECT().Read("Task/task1", gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ string, result *fhir.Task, _ interface{}) error {
					*result = fhirCreatedTask
					return nil
				})

			var result fhir.Task
			httpResponse := httptest.NewRecorder()
			err := ExecuteTransactionAndRespondWithEntry(fhirClient, tx.Bundle(), func(entry fhir.BundleEntry) bool {
				return *entry.Response.Location == "Task/task1"
			}, httpResponse, &result)

			require.NoError(t, err)
			require.Equal(t, fhirCreatedTask, result)
			responseData, _ := io.ReadAll(httpResponse.Body)
			require.JSONEq(t, `{"id":"task1", "intent":"order", "resourceType":"Task", "status":"completed"}`, string(responseData))
		})
		t.Run("caller not interested in result", func(t *testing.T) {
			fhirClient := mock.NewMockClient(ctrl)
			fhirClient.EXPECT().Create(tx.Bundle(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ fhir.Bundle, result *fhir.Bundle, _ interface{}) error {
					*result = fhirBundleResult
					return nil
				})
			fhirClient.EXPECT().Read("Task/task1", gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ string, result *map[string]interface{}, _ interface{}) error {
					*result = map[string]interface{}{
						"id": "task1",
					}
					return nil
				})

			httpResponse := httptest.NewRecorder()
			err := ExecuteTransactionAndRespondWithEntry(fhirClient, tx.Bundle(), func(entry fhir.BundleEntry) bool {
				return *entry.Response.Location == "Task/task1"
			}, httpResponse, nil)

			require.NoError(t, err)
			responseData, _ := io.ReadAll(httpResponse.Body)
			require.JSONEq(t, `{"id":"task1"}`, string(responseData))
		})

		t.Run("caller not interested in setting the response", func(t *testing.T) {
			fhirClient := mock.NewMockClient(ctrl)
			fhirClient.EXPECT().Create(tx.Bundle(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ fhir.Bundle, result *fhir.Bundle, _ interface{}) error {
					*result = fhirBundleResult
					return nil
				})
			fhirClient.EXPECT().Read("Task/task1", gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ string, result *fhir.Task, _ interface{}) error {
					*result = fhirCreatedTask
					return nil
				})

			var result fhir.Task
			err := ExecuteTransactionAndRespondWithEntry(fhirClient, tx.Bundle(), func(entry fhir.BundleEntry) bool {
				return *entry.Response.Location == "Task/task1"
			}, nil, &result)

			require.NoError(t, err)
			require.Equal(t, fhirCreatedTask, result)
		})
	})
}
