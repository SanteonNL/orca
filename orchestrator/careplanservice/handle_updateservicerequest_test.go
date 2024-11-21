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

func TestService_handleUpdateServiceRequest(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []HandleUpdateMetaBasedResourceTestStruct[fhir.ServiceRequest]{
		{
			ctx:           ctx,
			name:          "error: Meta is not present, can't determine ServiceRequest creator",
			expectedError: fmt.Errorf("cannot determine creator of ServiceRequest"),
			existingResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
		},
		{
			ctx:           ctx,
			name:          "error: request.Id != resource.Id",
			expectedError: coolfhir.BadRequestError("ID in request URL does not match ID in resource"),
			existingResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
					resource.Id = to.Ptr("999")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: attempting to update Meta.Source",
			expectedError: &coolfhir.ErrorWithCode{Message: "ServiceRequest Meta.Source cannot be changed", StatusCode: http.StatusForbidden},
			existingResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(request *fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
					resource.Meta = &fhir.Meta{
						Source: to.Ptr("some-other-org"),
					}
				},
			},
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:          "error: valid update, but requester does not have access",
			expectedError: &coolfhir.ErrorWithCode{Message: "requester does not have access to update ServiceRequest", StatusCode: http.StatusForbidden},
			existingResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
					resource.DoNotPerform = to.Ptr(true)
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: valid update but error reading from FHIR",
			expectedError: fmt.Errorf("failed to read ServiceRequest: %w", errors.New("fhir error: service unavailable")),
			errorFromRead: errors.New("fhir error: service unavailable"),
			// Setting this so the unit test doesn't fail, functionally it is not used
			existingResource: &fhir.ServiceRequest{
				Id: to.Ptr("1"),
			},
			request: []func(*fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
					resource.DoNotPerform = to.Ptr(true)
				},
			},
		},
		{
			ctx:  ctx,
			name: "error: cannot update BasedOn",
			existingResource: &fhir.ServiceRequest{
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
				Message:    "ServiceRequest.BasedOn cannot be updated",
				StatusCode: http.StatusBadRequest,
			},
			request: []func(*fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
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
			existingResource: &fhir.ServiceRequest{
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
			request: []func(*fhir.ServiceRequest){
				func(resource *fhir.ServiceRequest) {
					resource.DoNotPerform = to.Ptr(true)
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
		tt.resourceType = "ServiceRequest"
		tt.id = "1"
		testHelperHandleUpdateMetaBasedResource[fhir.ServiceRequest](t, tt, service.handleUpdateServiceRequest)
	}
}

func Test_validateServiceRequestUpdate(t *testing.T) {
	t.Run("ServiceRequest basedOn cannot be updated", func(t *testing.T) {
		a := &fhir.ServiceRequest{BasedOn: []fhir.Reference{{Reference: to.Ptr("CarePlan/1")}}}
		b := &fhir.ServiceRequest{BasedOn: []fhir.Reference{{Reference: to.Ptr("CarePlan/2")}}}
		err := validateServiceRequestUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "ServiceRequest.BasedOn cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("ServiceRequest subject cannot be updated", func(t *testing.T) {
		a := &fhir.ServiceRequest{Subject: fhir.Reference{Reference: to.Ptr("Patient/1")}}
		b := &fhir.ServiceRequest{Subject: fhir.Reference{Reference: to.Ptr("Patient/2")}}
		err := validateServiceRequestUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "ServiceRequest.Subject cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("ServiceRequest requester cannot be updated", func(t *testing.T) {
		a := &fhir.ServiceRequest{Requester: &fhir.Reference{Reference: to.Ptr("Practitioner/1")}}
		b := &fhir.ServiceRequest{Requester: &fhir.Reference{Reference: to.Ptr("Practitioner/2")}}
		err := validateServiceRequestUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "ServiceRequest.Requester cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("ServiceRequest performer cannot be updated", func(t *testing.T) {
		a := &fhir.ServiceRequest{Performer: []fhir.Reference{{Reference: to.Ptr("Practitioner/1")}}}
		b := &fhir.ServiceRequest{Performer: []fhir.Reference{{Reference: to.Ptr("Practitioner/2")}}}
		err := validateServiceRequestUpdate(a, b)
		require.Error(t, err)
		require.Equal(t, "ServiceRequest.Performer cannot be updated", err.(*coolfhir.ErrorWithCode).Message)
	})
	t.Run("valid update", func(t *testing.T) {
		a := &fhir.ServiceRequest{DoNotPerform: to.Ptr(false)}
		b := &fhir.ServiceRequest{DoNotPerform: to.Ptr(true)}
		err := validateServiceRequestUpdate(a, b)
		require.NoError(t, err)
	})
}
