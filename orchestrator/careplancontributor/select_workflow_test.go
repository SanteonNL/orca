package careplancontributor

import (
	"context"
	"fmt"
	"testing"

	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/taskengine"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

// stubWorkflowProvider returns a fixed result per condition code, regardless of the service code.
type stubWorkflowProvider map[string]func() (*taskengine.Workflow, error)

func (s stubWorkflowProvider) Provide(_ context.Context, _ fhir.Coding, conditionCode fhir.Coding) (*taskengine.Workflow, error) {
	if result, ok := s[*conditionCode.Code]; ok {
		return result()
	}
	return nil, fmt.Errorf("%w: unknown condition", taskengine.ErrWorkflowNotFound)
}

func (s stubWorkflowProvider) QuestionnaireLoader() taskengine.QuestionnaireLoader {
	return taskengine.TestQuestionnaireLoader{}
}

func TestService_selectWorkflow(t *testing.T) {
	workflow := &taskengine.Workflow{Steps: []taskengine.WorkflowStep{{QuestionnaireUrl: "Questionnaire/1"}}}
	found := func() (*taskengine.Workflow, error) { return workflow, nil }
	notFound := func() (*taskengine.Workflow, error) {
		return nil, fmt.Errorf("%w: no healthcare services found", taskengine.ErrWorkflowNotFound)
	}
	ambiguous := func() (*taskengine.Workflow, error) {
		return nil, fmt.Errorf("%w: expected 1 questionnaire, got 2", taskengine.ErrMultipleWorkflows)
	}
	// Task with 2 condition codings (like Epic sends), for 1 service coding.
	task := &fhir.Task{
		Id:    to.Ptr("task-1"),
		Focus: &fhir.Reference{Reference: to.Ptr("ServiceRequest/1")},
		ReasonCode: &fhir.CodeableConcept{Coding: []fhir.Coding{
			{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("A")},
			{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("B")},
		}},
	}
	newService := func(t *testing.T, provider stubWorkflowProvider) (*Service, *mock.MockClient) {
		cpsClient := mock.NewMockClient(gomock.NewController(t))
		cpsClient.EXPECT().Read("ServiceRequest/1", gomock.Any()).DoAndReturn(func(_ string, target any, _ ...any) error {
			*target.(*fhir.ServiceRequest) = fhir.ServiceRequest{
				Id: to.Ptr("1"),
				Code: &fhir.CodeableConcept{Coding: []fhir.Coding{
					{System: to.Ptr("http://snomed.info/sct"), Code: to.Ptr("719858009")},
				}},
			}
			return nil
		})
		return &Service{workflows: provider}, cpsClient
	}

	t.Run("one combination not found, other resolves", func(t *testing.T) {
		service, cpsClient := newService(t, stubWorkflowProvider{"A": notFound, "B": found})
		actual, err := service.selectWorkflow(context.Background(), cpsClient, task)
		require.NoError(t, err)
		require.Same(t, workflow, actual)
	})
	t.Run("one combination ambiguous, other resolves", func(t *testing.T) {
		service, cpsClient := newService(t, stubWorkflowProvider{"A": ambiguous, "B": found})
		actual, err := service.selectWorkflow(context.Background(), cpsClient, task)
		require.NoError(t, err)
		require.Same(t, workflow, actual)
	})
	t.Run("no combination resolves", func(t *testing.T) {
		service, cpsClient := newService(t, stubWorkflowProvider{"A": notFound, "B": notFound})
		_, err := service.selectWorkflow(context.Background(), cpsClient, task)
		require.EqualError(t, err, "ServiceRequest.code and Task.reason.code does not match any workflows (task=task-1)")
	})
	t.Run("no combination resolves, some ambiguous", func(t *testing.T) {
		service, cpsClient := newService(t, stubWorkflowProvider{"A": notFound, "B": ambiguous})
		_, err := service.selectWorkflow(context.Background(), cpsClient, task)
		require.EqualError(t, err, "ServiceRequest.code and Task.reason.code does not match any workflows, 1 combination(s) matched multiple workflows (task=task-1)")
	})
}
