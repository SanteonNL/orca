package careplancontributor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
var serviceRequestBundle fhir.Bundle

func init() {
	var err error
	if serviceRequestBundleJSON, err = os.ReadFile("test/servicerequest-bundle.json"); err != nil {
		panic(err)
	}
	if err = json.Unmarshal(serviceRequestBundleJSON, &serviceRequestBundle); err != nil {
		panic(err)
	}
}

func TestService_confirm(t *testing.T) {
	carePlanService := startCarePlanService(t)
	service := Service{
		SessionManager:  user.NewSessionManager(),
		CarePlanService: carePlanService,
	}
	localFHIR := startLocalFHIRServer(t)

	task, err := service.confirm(localFHIR, "ServiceRequest/1", "Patient/1")

	require.NoError(t, err)
	require.NotNil(t, task)
}

func startLocalFHIRServer(t *testing.T) fhirclient.Client {
	mux := http.NewServeMux()
	var serviceRequestURL string
	mux.HandleFunc("GET /ServiceRequest", func(w http.ResponseWriter, r *http.Request) {
		serviceRequestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(serviceRequestBundleJSON)
	})
	mux.HandleFunc("GET /ServiceRequest/1", func(w http.ResponseWriter, r *http.Request) {
		serviceRequestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(serviceRequestBundleJSON)
	})
	mux.HandleFunc("GET /Patient/1", func(w http.ResponseWriter, r *http.Request) {
		serviceRequestURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(serviceRequestBundleJSON)
	})
	println(serviceRequestURL)
	httpServer := httptest.NewServer(mux)
	baseURL, _ := url.Parse(httpServer.URL)
	return fhirclient.New(baseURL, httpServer.Client())
}

func Test_unmarshalFromBundle(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		var serviceRequest fhir.ServiceRequest
		err := unmarshalFromBundle(serviceRequestBundle, "ServiceRequest", &serviceRequest)
		require.NoError(t, err)
		require.NotNil(t, serviceRequest)
	})
}

func startCarePlanService(t *testing.T) fhirclient.Client {
	mux := http.NewServeMux()
	httpServer := httptest.NewServer(mux)
	mux.HandleFunc("POST /Task", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"id":"task-1"}`))
	})
	baseURL, _ := url.Parse(httpServer.URL)
	return fhirclient.New(baseURL, httpServer.Client())
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
