package careplanservice

import (
	"context"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/http"
	"testing"
)

func TestService_handleUpdateQuestionnaireResponse(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []HandleUpdateMetaBasedResourceTestStruct[fhir.QuestionnaireResponse]{
		{
			ctx:           ctx,
			name:          "error: Meta is not present, can't determine QuestionnaireResponse creator",
			expectedError: fmt.Errorf("cannot determine creator of QuestionnaireResponse"),
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
			},
		},
		{
			ctx:           ctx,
			name:          "error: request.Id != resource.Id",
			expectedError: coolfhir.BadRequestError("ID in request URL does not match ID in resource"),
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.Id = to.Ptr("999")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: attempting to update Meta.Source",
			expectedError: &coolfhir.ErrorWithCode{Message: "QuestionnaireResponse Meta.Source cannot be changed", StatusCode: http.StatusForbidden},
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.Meta = &fhir.Meta{
						Source: to.Ptr("some-other-org"),
					}
				},
			},
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:          "error: valid update, but requester does not have access",
			expectedError: &coolfhir.ErrorWithCode{Message: "requester does not have access to update QuestionnaireResponse", StatusCode: http.StatusForbidden},
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.Status = fhir.QuestionnaireResponseStatusCompleted
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: valid update but error reading from FHIR",
			expectedError: fmt.Errorf("failed to read QuestionnaireResponse: %w", errors.New("fhir error: service unavailable")),
			errorFromRead: errors.New("fhir error: service unavailable"),
			// Setting this so the unit test doesn't fail, functionally it is not used
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
			},
			request: []func(*fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.Status = fhir.QuestionnaireResponseStatusCompleted
				},
			},
		},
		{
			ctx:  ctx,
			name: "error: cannot update BasedOn",
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("CarePlan/1"),
					},
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "QuestionnaireResponse fields other than Status and Item cannot be updated",
				StatusCode: http.StatusBadRequest,
			},
			request: []func(*fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.BasedOn = []fhir.Reference{
						{
							Reference: to.Ptr("CarePlan/2"),
						},
					}
				},
			},
		},
		{
			ctx:  ctx,
			name: "success: valid update",
			existingResource: &fhir.QuestionnaireResponse{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
				BasedOn: []fhir.Reference{
					{
						Reference: to.Ptr("CarePlan/1"),
					},
				},
			},
			request: []func(*fhir.QuestionnaireResponse){
				func(resource *fhir.QuestionnaireResponse) {
					resource.Status = fhir.QuestionnaireResponseStatusCompleted
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
		tt.resourceType = "QuestionnaireResponse"
		tt.id = "1"
		testHelperHandleUpdateMetaBasedResource[fhir.QuestionnaireResponse](t, tt, service.handleUpdateQuestionnaireResponse)
	}
}

func Test_validateQuestionnaireResponseUpdate(t *testing.T) {
	t.Run("QuestionnaireResponse basedOn cannot be updated", func(t *testing.T) {
		a := &fhir.QuestionnaireResponse{BasedOn: []fhir.Reference{{Reference: to.Ptr("CarePlan/1")}}}
		b := &fhir.QuestionnaireResponse{BasedOn: []fhir.Reference{{Reference: to.Ptr("CarePlan/2")}}}
		err := validateQuestionnaireResponseUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "QuestionnaireResponse fields other than Status and Item cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("QuestionnaireResponse subject cannot be updated", func(t *testing.T) {
		a := &fhir.QuestionnaireResponse{Subject: &fhir.Reference{Reference: to.Ptr("Patient/1")}}
		b := &fhir.QuestionnaireResponse{Subject: &fhir.Reference{Reference: to.Ptr("Patient/2")}}
		err := validateQuestionnaireResponseUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "QuestionnaireResponse fields other than Status and Item cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("QuestionnaireResponse status can be updated", func(t *testing.T) {
		a := &fhir.QuestionnaireResponse{Status: fhir.QuestionnaireResponseStatusInProgress}
		b := &fhir.QuestionnaireResponse{Status: fhir.QuestionnaireResponseStatusCompleted}
		err := validateQuestionnaireResponseUpdate(a, b)
		require.NoError(t, err)
	})
	t.Run("QuestionnaireResponse item can be updated", func(t *testing.T) {
		a := &fhir.QuestionnaireResponse{Item: []fhir.QuestionnaireResponseItem{{LinkId: "1"}}}
		b := &fhir.QuestionnaireResponse{Item: []fhir.QuestionnaireResponseItem{{LinkId: "2"}}}
		err := validateQuestionnaireResponseUpdate(a, b)
		require.NoError(t, err)
	})
}
