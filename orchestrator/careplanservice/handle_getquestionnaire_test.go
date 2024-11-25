package careplanservice

import (
	"context"
	"errors"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"testing"
)

func TestService_handleGetQuestionnaire(t *testing.T) {
	tests := []TestStruct[fhir.Questionnaire]{
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "error: Questionnaire does not exist",
			id:            "1",
			resourceType:  "Questionnaire",
			errorFromRead: errors.New("fhir error: Questionnaire not found"),
			expectedError: errors.New("fhir error: Questionnaire not found"),
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal1),
			name:          "ok: Questionnaire exists, auth",
			id:            "1",
			resourceType:  "Questionnaire",
			expectedError: nil,
			returnedResource: &fhir.Questionnaire{
				Id: to.Ptr("1"),
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create a mock FHIR client using the generated mock
	mockFHIRClient := mock.NewMockClient(ctrl)

	// Create the service with the mock FHIR client
	service := &Service{
		fhirClient: mockFHIRClient,
	}

	for _, tt := range tests {
		tt.mockClient = mockFHIRClient
		testHelperHandleGetResource[fhir.Questionnaire](t, tt, service.handleGetQuestionnaire)
	}
}
