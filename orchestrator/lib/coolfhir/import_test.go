package coolfhir

import (
	"context"
	"encoding/json"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestImportResources(t *testing.T) {
	// Start up test HTTP server that serves JSON FHIR Bundles
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tasks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		result := (&BundleBuilder{
			Type: fhir.BundleTypeBatch,
		}).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task/1",
				Method: fhir.HTTPVerbPOST,
			}, nil).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task/2",
				Method: fhir.HTTPVerbPOST,
			}, nil).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task?id=1",
				Method: fhir.HTTPVerbPOST,
			}, nil).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task/?id=1",
				Method: fhir.HTTPVerbPOST,
			}, nil).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task",
				Method: fhir.HTTPVerbPOST,
			}, nil)
		_ = json.NewEncoder(w).Encode(result.Bundle())
	})
	mux.HandleFunc("GET /bundle-with-other-resources", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		result := (&BundleBuilder{
			Type: fhir.BundleTypeBatch,
		}).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "Task/1",
				Method: fhir.HTTPVerbPOST,
			}, nil).
			Append(fhir.Patient{}, &fhir.BundleEntryRequest{
				Url:    "Patient/2",
				Method: fhir.HTTPVerbPOST,
			}, nil)
		_ = json.NewEncoder(w).Encode(result.Bundle())
	})
	mux.HandleFunc("GET /bundle-with-absolute-url", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		result := (&BundleBuilder{
			Type: fhir.BundleTypeBatch,
		}).
			Append(fhir.Task{}, &fhir.BundleEntryRequest{
				Url:    "http://example.com/fhir/Task/1",
				Method: fhir.HTTPVerbPOST,
			}, nil)
		_ = json.NewEncoder(w).Encode(result.Bundle())
	})
	mux.HandleFunc("GET /invalid-json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{`))
	})
	mux.HandleFunc("GET /not-a-fhir-bundle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"resourceType": "Patient"}`))
	})
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	t.Run("ok - read from URL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		client := mock.NewMockClient(ctrl)
		client.EXPECT().CreateWithContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		err := ImportResources(context.Background(), client, []string{"Task"}, httpServer.URL+"/tasks")
		require.NoError(t, err)
	})
	t.Run("can't load URL", func(t *testing.T) {
		err := ImportResources(context.Background(), nil, []string{"Task"}, httpServer.URL+"/not-found")
		require.EqualError(t, err, "unexpected status code: 404")
	})
	t.Run("invalid JSON", func(t *testing.T) {
		err := ImportResources(context.Background(), nil, []string{"Task"}, httpServer.URL+"/invalid-json")
		require.ErrorContains(t, err, "invalid resource of type")
	})
	t.Run("not a FHIR Bundle", func(t *testing.T) {
		err := ImportResources(context.Background(), nil, []string{"Task"}, httpServer.URL+"/not-a-fhir-bundle")
		require.ErrorContains(t, err, "not a FHIR Bundle")
	})
	t.Run("bundle contains entry with full URL", func(t *testing.T) {
		err := ImportResources(context.Background(), nil, []string{"Task"}, httpServer.URL+"/bundle-with-absolute-url")
		require.ErrorContains(t, err, "entry contains a full URL, which is not supported")
	})
	t.Run("bundle contains other resources than allowed", func(t *testing.T) {
		err := ImportResources(context.Background(), nil, []string{"Task"}, httpServer.URL+"/bundle-with-other-resources")
		require.ErrorContains(t, err, "entry contains a resource of an unexpected type")
	})
	t.Run("ok - read from file", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		client := mock.NewMockClient(ctrl)
		client.EXPECT().CreateWithContext(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

		err := ImportResources(context.Background(), client, []string{"Task"}, "file://testdata/bundle.json")
		require.NoError(t, err)
	})
}
