package careplancontributor

import (
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/applaunch/clients"
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

func TestService_ProxyToEHR(t *testing.T) {
	// Test that the service registers the EHR FHIR proxy URL that proxies to the backing FHIR server of the EHR
	// Setup: configure backing EHR FHIR server to which the service proxies
	fhirServerMux := http.NewServeMux()
	capturedHost := ""
	fhirServerMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
	})
	fhirServer := httptest.NewServer(fhirServerMux)
	fhirServerURL, _ := url.Parse(fhirServer.URL)
	fhirServerURL.Path = "/fhir"
	// Setup: create the service

	clients.Factories["test"] = func(properties map[string]string) clients.ClientProperties {
		return clients.ClientProperties{
			Client:  fhirServer.Client().Transport,
			BaseURL: fhirServerURL,
		}
	}
	sessionManager, sessionID := createTestSession()

	service := New(Config{}, sessionManager, nil)
	// Setup: configure the service to proxy to the backing FHIR server
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/ehr/fhir/Patient", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, fhirServerURL.Host, capturedHost)
}

func TestService_ProxyToCPS(t *testing.T) {
	// Test that the service registers the CarePlanService FHIR proxy URL that proxies to the CarePlanService
	// Setup: configure CarePlanService to which the service proxies
	carePlanServiceMux := http.NewServeMux()
	capturedHost := ""
	carePlanServiceMux.HandleFunc("GET /fhir/Patient", func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		capturedHost = request.Host
	})
	carePlanService := httptest.NewServer(carePlanServiceMux)
	carePlanServiceURL, _ := url.Parse(carePlanService.URL)
	carePlanServiceURL.Path = "/fhir"

	clients.Factories["test"] = func(properties map[string]string) clients.ClientProperties {
		return clients.ClientProperties{
			Client:  carePlanService.Client().Transport,
			BaseURL: carePlanServiceURL,
		}
	}
	sessionManager, sessionID := createTestSession()

	service := New(Config{
		CarePlanService: CarePlanServiceConfig{
			URL: carePlanServiceURL.String(),
		},
	}, sessionManager, nil)
	// Setup: configure the service to proxy to the upstream CarePlanService
	frontServerMux := http.NewServeMux()
	service.RegisterHandlers(frontServerMux)
	frontServer := httptest.NewServer(frontServerMux)

	httpRequest, _ := http.NewRequest("GET", frontServer.URL+"/contrib/cps/fhir/Patient", nil)
	httpRequest.AddCookie(&http.Cookie{
		Name:  "sid",
		Value: sessionID,
	})
	httpResponse, err := frontServer.Client().Do(httpRequest)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, httpResponse.StatusCode)
	require.Equal(t, carePlanServiceURL.Host, capturedHost)
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

func TestService_handleGetContext(t *testing.T) {
	httpResponse := httptest.NewRecorder()
	Service{}.handleGetContext(httpResponse, nil, &user.SessionData{
		Values: map[string]string{
			"test":           "value",
			"practitioner":   "the-doctor",
			"serviceRequest": "ServiceRequest/1",
			"patient":        "Patient/1",
		},
	})
	assert.Equal(t, http.StatusOK, httpResponse.Code)
	assert.JSONEq(t, `{
		"practitioner": "the-doctor",
		"serviceRequest": "ServiceRequest/1",	
		"patient": "Patient/1"
	}`, httpResponse.Body.String())
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

func createTestSession() (*user.SessionManager, string) {
	sessionManager := user.NewSessionManager()
	sessionHttpResponse := httptest.NewRecorder()
	sessionManager.Create(sessionHttpResponse, user.SessionData{
		FHIRLauncher: "test",
	})
	// extract session ID; sid=<something>;
	cookieValue := sessionHttpResponse.Header().Get("Set-Cookie")
	cookieValue = strings.Split(cookieValue, ";")[0]
	cookieValue = strings.Split(cookieValue, "=")[1]
	return sessionManager, cookieValue
}
