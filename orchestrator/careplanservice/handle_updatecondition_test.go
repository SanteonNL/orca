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

func TestService_handleUpdateCondition(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []HandleUpdateMetaBasedResourceTestStruct[fhir.Condition]{
		{
			ctx:           ctx,
			name:          "error: Meta is not present, can't determine Condition creator",
			expectedError: fmt.Errorf("cannot determine creator of Condition"),
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
			},
		},
		{
			ctx:           ctx,
			name:          "error: request.Id != resource.Id",
			expectedError: coolfhir.BadRequestError("ID in request URL does not match ID in resource"),
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.Condition){
				func(resource *fhir.Condition) {
					resource.Id = to.Ptr("999")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: attempting to update Meta.Source",
			expectedError: &coolfhir.ErrorWithCode{Message: "Condition Meta.Source cannot be changed", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.Condition){
				func(resource *fhir.Condition) {
					resource.Meta = &fhir.Meta{
						Source: to.Ptr("some-other-org"),
					}
				},
			},
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:          "error: valid update, but requester does not have access",
			expectedError: &coolfhir.ErrorWithCode{Message: "requester does not have access to update Condition", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Condition){
				func(resource *fhir.Condition) {
					resource.AbatementString = to.Ptr("some abatement")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: valid update but error reading from FHIR",
			expectedError: fmt.Errorf("failed to read Condition: %w", errors.New("fhir error: service unavailable")),
			errorFromRead: errors.New("fhir error: service unavailable"),
			// Setting this so the unit test doesn't fail, functionally it is not used
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
			},
			request: []func(*fhir.Condition){
				func(resource *fhir.Condition) {
					resource.AbatementString = to.Ptr("some abatement")
				},
			},
		},
		{
			ctx:  ctx,
			name: "error: cannot update Subject",
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
				Subject: fhir.Reference{
					Reference: to.Ptr("Patient/1"),
				},
			},
			expectedError: &coolfhir.ErrorWithCode{
				Message:    "Condition.Subject cannot be updated",
				StatusCode: http.StatusBadRequest,
			},
			request: []func(*fhir.Condition){
				func(resource *fhir.Condition) {
					resource.Subject = fhir.Reference{
						Reference: to.Ptr("Patient/2"),
					}
				},
			},
		},
		{
			ctx:  ctx,
			name: "success: valid update",
			existingResource: &fhir.Condition{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Condition){
				func(resource *fhir.Condition) {
					resource.AbatementString = to.Ptr("some abatement")
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
		tt.resourceType = "Condition"
		tt.id = "1"
		testHelperHandleUpdateMetaBasedResource[fhir.Condition](t, tt, service.handleUpdateCondition)
	}
}

func Test_validateConditionUpdate(t *testing.T) {
	t.Run("Condition subject cannot be updated", func(t *testing.T) {
		a := &fhir.Condition{Subject: fhir.Reference{Reference: to.Ptr("Patient/1")}}
		b := &fhir.Condition{Subject: fhir.Reference{Reference: to.Ptr("Patient/2")}}
		err := validateConditionUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "Condition.Subject cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("valid update", func(t *testing.T) {
		a := &fhir.Condition{AbatementString: to.Ptr("some abatement")}
		b := &fhir.Condition{AbatementString: to.Ptr("some abatement")}
		err := validateConditionUpdate(a, b)
		require.NoError(t, err)
	})
}
