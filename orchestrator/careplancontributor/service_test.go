package careplancontributor

import (
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/user"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
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
