package taskengine

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestMemoryFHIRResourcesWorkflowProvider_Provide(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.Contains(request.URL.String(), "healthcareservice") {
			data, err := os.ReadFile("testdata/healthcareservice-bundle.json")
			if err != nil {
				panic(err)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(data)
		} else if strings.Contains(request.URL.String(), "questionnaire") {
			data, err := os.ReadFile("testdata/questionnaire-bundle.json")
			if err != nil {
				panic(err)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(data)
		} else {
			writer.WriteHeader(http.StatusNotFound)
		}
	}))

	provider := &MemoryWorkflowProvider{}
	require.NoError(t, provider.LoadBundle(context.Background(), httpServer.URL+"/healthcareservice"))
	require.NoError(t, provider.LoadBundle(context.Background(), httpServer.URL+"/questionnaire"))

	serviceCode := fhir.Coding{
		System: to.Ptr("http://snomed.info/sct"),
		Code:   to.Ptr("719858009"),
	}
	conditionCode := fhir.Coding{
		System: to.Ptr("http://snomed.info/sct"),
		Code:   to.Ptr("84114007"),
	}

	t.Run("provide workflow", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.NoError(t, err)
			require.NotNil(t, workflow)
		})
		t.Run("service not supported", func(t *testing.T) {
			serviceCode := fhir.Coding{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("134"),
			}
			_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
		})
		t.Run("condition not supported", func(t *testing.T) {
			conditionCode := fhir.Coding{
				System: to.Ptr("http://snomed.info/sct"),
				Code:   to.Ptr("1234"),
			}
			_, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.ErrorIs(t, err, ErrWorkflowNotFound)
		})
	})
	t.Run("load questionnaire", func(t *testing.T) {
		t.Run("ok", func(t *testing.T) {
			workflow, err := provider.Provide(context.Background(), serviceCode, conditionCode)
			require.NoError(t, err)
			require.NotNil(t, workflow)

			questionnaire, err := provider.QuestionnaireLoader().Load(context.Background(), workflow.Start().QuestionnaireUrl)
			require.NoError(t, err)
			require.NotNil(t, questionnaire)
		})
	})
}
