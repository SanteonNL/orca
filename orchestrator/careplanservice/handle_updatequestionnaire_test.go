package careplanservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"testing"
)

func TestService_handleUpdateQuestionnaire(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []HandleUpdateMetaBasedResourceTestStruct[fhir.Questionnaire]{
		{
			ctx:           ctx,
			name:          "error: Meta is not present, can't determine questionnaire creator",
			expectedError: fmt.Errorf("cannot determine creator of Questionnaire"),
			existingResource: &fhir.Questionnaire{
				Id:          to.Ptr("1"),
				Description: to.Ptr("A description"),
			},
		},
		{
			ctx:           ctx,
			name:          "error: request.Id != resource.Id",
			expectedError: coolfhir.BadRequestError("ID in request URL does not match ID in resource"),
			existingResource: &fhir.Questionnaire{
				Id:          to.Ptr("1"),
				Description: to.Ptr("A description"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Questionnaire){
				func(resource *fhir.Questionnaire) {
					resource.Id = to.Ptr("999")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: attempting to update Meta.Source",
			expectedError: &coolfhir.ErrorWithCode{Message: "Questionnaire Meta.Source cannot be changed", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Questionnaire{
				Id:          to.Ptr("1"),
				Description: to.Ptr("A description"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Questionnaire){
				func(resource *fhir.Questionnaire) {
					resource.Meta = &fhir.Meta{
						Source: to.Ptr("some-other-org"),
					}
				},
			},
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:          "error: valid update, but requester does not have access",
			expectedError: &coolfhir.ErrorWithCode{Message: "requester does not have access to update Questionnaire", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Questionnaire{
				Id:          to.Ptr("1"),
				Description: to.Ptr("A description"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Questionnaire){
				func(resource *fhir.Questionnaire) {
					resource.Url = to.Ptr("http://example.com")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: valid update but error reading from FHIR",
			expectedError: fmt.Errorf("failed to read Questionnaire: %w", errors.New("fhir error: service unavailable")),
			errorFromRead: errors.New("fhir error: service unavailable"),
			// Setting this so the unit test doesn't fail, functionally it is not used
			existingResource: &fhir.Questionnaire{
				Id: to.Ptr("1"),
			},
			request: []func(*fhir.Questionnaire){
				func(resource *fhir.Questionnaire) {
					resource.Url = to.Ptr("http://example.com")
				},
			},
		},
		{
			ctx:  ctx,
			name: "success: valid update",
			existingResource: &fhir.Questionnaire{
				Id:          to.Ptr("1"),
				Description: to.Ptr("A description"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Questionnaire){
				func(resource *fhir.Questionnaire) {
					resource.Url = to.Ptr("http://example.com")
				},
			},
		},
	}
	for _, tt := range tests {
		ctrl := gomock.NewController(t)
		mockFhirClient := mock.NewMockClient(ctrl)
		service := &Service{
			fhirClient: mockFhirClient,
		}

		tt.mockClient = mockFhirClient
		tt.resourceType = "Questionnaire"
		tt.id = "1"
		testHelperHandleUpdateMetaBasedResource[fhir.Questionnaire](t, tt, service.handleUpdateQuestionnaire)
	}
}
