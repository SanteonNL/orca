package careplancontributor

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	mock_fhirclient "github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var serviceRequestBundleJSON []byte

func init() {
	var err error
	if serviceRequestBundleJSON, err = os.ReadFile("test/servicerequest-bundle.json"); err != nil {
		panic(err)
	}
}

func TestService_confirm(t *testing.T) {
	carePlanService, task, carePlan := startCarePlanService(t)
	service := Service{
		SessionManager:  user.NewSessionManager(),
		CarePlanService: carePlanService,
	}
	localFHIR := startLocalFHIRServer(t)

	err := service.confirm(localFHIR, "ServiceRequest/1", "Patient/1")

	require.NoError(t, err)
	t.Run("CarePlan has been created", func(t *testing.T) {
		require.NotNil(t, carePlan)
		t.Run("CarePlan.subject refers to Patient using BSN", func(t *testing.T) {
			require.NotNil(t, carePlan.Subject)
			assert.Equal(t, "Patient", *carePlan.Subject.Type)
			require.NotNil(t, carePlan.Subject.Identifier)
			assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/bsn", *carePlan.Subject.Identifier.System)
			require.NotNil(t, carePlan.Subject.Identifier.System)
			assert.Equal(t, "111222333", *carePlan.Subject.Identifier.Value)
			require.NotNil(t, carePlan.Subject.Identifier.Value)
		})
	})
	t.Run("Task has been created", func(t *testing.T) {
		require.NotNil(t, task)
		assert.Equal(t, fhir.TaskStatusAccepted, task.Status)
		t.Run("Task.basedOn refers to CarePlan", func(t *testing.T) {
			require.Len(t, task.BasedOn, 1)
			assert.Equal(t, "CarePlan/"+*carePlan.Id, *task.BasedOn[0].Reference)
		})
		// Task.input[0]
		require.Len(t, task.Input, 1)
		t.Run("Task.for contains a reference to a contained Patient resource", func(t *testing.T) {
			require.NotNil(t, task.For)
			assert.Equal(t, "Patient", *task.For.Type)
			assert.True(t, strings.HasPrefix(*task.For.Reference, "#"))
		})
	})
}

func startLocalFHIRServer(t *testing.T) fhirclient.Client {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ServiceRequest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(serviceRequestBundleJSON)
	})
	mux.HandleFunc("GET /ServiceRequest/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(serviceRequestBundleJSON)
	})
	mux.HandleFunc("GET /Patient/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		data, _ := os.ReadFile("test/patient.json")
		_, _ = w.Write(data)
	})
	httpServer := httptest.NewServer(mux)
	baseURL, _ := url.Parse(httpServer.URL)
	return fhirclient.New(baseURL, httpServer.Client(), coolfhir.Config())
}

func startCarePlanService(t *testing.T) (fhirclient.Client, *fhir.Task, *fhir.CarePlan) {
	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	var task = new(fhir.Task)
	var carePlan = new(fhir.CarePlan)
	mux.HandleFunc("POST /Task", func(writer http.ResponseWriter, request *http.Request) {
		var newTask fhir.Task
		data, _ := io.ReadAll(request.Body)
		if err := json.Unmarshal(data, &newTask); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		newTask.Id = to.Ptr("1")
		if len(carePlan.Activity) == 0 {
			carePlan.Activity = append(carePlan.Activity, fhir.CarePlanActivity{
				Reference: &fhir.Reference{
					Reference: to.Ptr("Task/" + *newTask.Id),
					Type:      to.Ptr("Task"),
				},
			})
		}
		newTask.Status = fhir.TaskStatusAccepted // make test simpler by setting status to accepted
		*task = newTask
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		data, _ = json.Marshal(*task)
		_, _ = writer.Write(data)
	})
	mux.HandleFunc("GET /Task/1", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		data, _ := json.Marshal(*task)
		_, _ = writer.Write(data)
	})
	mux.HandleFunc("PUT /Task/1", func(writer http.ResponseWriter, request *http.Request) {
		data, _ := io.ReadAll(request.Body)
		if err := json.Unmarshal(data, task); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}

		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(data)
	})
	mux.HandleFunc("POST /CarePlan", func(writer http.ResponseWriter, request *http.Request) {
		var newCarePlan fhir.CarePlan
		data, _ := io.ReadAll(request.Body)
		if err := json.Unmarshal(data, &newCarePlan); err != nil {
			http.Error(writer, err.Error(), http.StatusBadRequest)
			return
		}
		newCarePlan.Id = to.Ptr("2")
		*carePlan = newCarePlan
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		data, _ = json.Marshal(*carePlan)
		_, _ = writer.Write(data)
	})

	baseURL, _ := url.Parse(httpServer.URL)
	return fhirclient.New(baseURL, httpServer.Client(), coolfhir.Config()), task, carePlan
}

func Test_shouldStopPollingOnAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockCarePlanService := mock_fhirclient.NewMockClient(ctrl)
	service := Service{CarePlanService: mockCarePlanService}

	taskID := "test-task-id"

	// First call returns a task that is not accepted
	firstTask := fhir.Task{Status: fhir.TaskStatusInProgress}
	mockCarePlanService.EXPECT().Read("Task/"+taskID, gomock.Any(), gomock.Any()).DoAndReturn(func(resource string, v *fhir.Task, opts ...interface{}) error {
		*v = firstTask
		return nil
	}).Times(1)

	// Second call returns a task that is accepted
	secondTask := fhir.Task{Status: fhir.TaskStatusAccepted}
	mockCarePlanService.EXPECT().Read("Task/"+taskID, gomock.Any(), gomock.Any()).DoAndReturn(func(resource string, v *fhir.Task, opts ...interface{}) error {
		*v = secondTask
		return nil
	}).Times(1)

	err := service.pollTaskStatus(taskID)
	assert.NoError(t, err)
}
