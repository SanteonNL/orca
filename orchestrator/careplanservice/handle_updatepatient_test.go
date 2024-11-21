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

func TestService_handleUpdatePatient(t *testing.T) {
	ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)

	tests := []HandleUpdateMetaBasedResourceTestStruct[fhir.Patient]{
		{
			ctx:           ctx,
			name:          "error: Meta is not present, can't determine Patient creator",
			expectedError: fmt.Errorf("cannot determine creator of Patient"),
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
			},
		},
		{
			ctx:           ctx,
			name:          "error: request.Id != resource.Id",
			expectedError: coolfhir.BadRequestError("ID in request URL does not match ID in resource"),
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Patient){
				func(resource *fhir.Patient) {
					resource.Id = to.Ptr("999")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: attempting to update Meta.Source",
			expectedError: &coolfhir.ErrorWithCode{Message: "Patient Meta.Source cannot be changed", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Patient){
				func(resource *fhir.Patient) {
					resource.Meta = &fhir.Meta{
						Source: to.Ptr("some-other-org"),
					}
				},
			},
		},
		{
			ctx:           auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			name:          "error: valid update, but requester does not have access",
			expectedError: &coolfhir.ErrorWithCode{Message: "requester does not have access to update Patient", StatusCode: http.StatusForbidden},
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Patient){
				func(resource *fhir.Patient) {
					resource.Language = to.Ptr("Engels")
				},
			},
		},
		{
			ctx:           ctx,
			name:          "error: valid update but error reading from FHIR",
			expectedError: fmt.Errorf("failed to read Patient: %w", errors.New("fhir error: service unavailable")),
			errorFromRead: errors.New("fhir error: service unavailable"),
			// Setting this so the unit test doesn't fail, functionally it is not used
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
			},
			request: []func(*fhir.Patient){
				func(resource *fhir.Patient) {
					resource.Language = to.Ptr("Engels")
				},
			},
		},
		{
			ctx:  ctx,
			name: "success: valid update",
			existingResource: &fhir.Patient{
				Id:       to.Ptr("1"),
				Language: to.Ptr("Nederlands"),
				Meta: &fhir.Meta{
					Source: to.Ptr(getOrgRef(*auth.TestPrincipal1)),
				},
			},
			request: []func(*fhir.Patient){
				func(resource *fhir.Patient) {
					resource.Language = to.Ptr("Engels")
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
		tt.resourceType = "Patient"
		tt.id = "1"
		testHelperHandleUpdateMetaBasedResource[fhir.Patient](t, tt, service.handleUpdatePatient)
	}
}
