package careplanservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"testing"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
)

func Test_handleCreateServiceRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	defaultReturnedBundle := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("ServiceRequest/1"),
					Status:   "201 Created",
				},
			},
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("AuditEvent/2"),
					Status:   "201 Created",
				},
			},
		},
	}

	defaultServiceRequest := fhir.ServiceRequest{
		Status: fhir.RequestStatusActive,
		Intent: fhir.RequestIntentOrder,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("123456"),
					Display: to.Ptr("Test Service"),
				},
			},
		},
		Requester: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("Organization"),
				Value:  to.Ptr("1234567890"),
			},
		},
	}
	defaultServiceRequestJSON, _ := json.Marshal(defaultServiceRequest)

	tests := []struct {
		name                        string
		serviceRequestToCreate      fhir.ServiceRequest
		createdServiceRequestBundle *fhir.Bundle
		returnedBundle              *fhir.Bundle
		errorFromCreate             error
		expectError                 error
		ctx                         context.Context
	}{
		{
			name:                   "happy flow - success",
			serviceRequestToCreate: defaultServiceRequest,
			createdServiceRequestBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: defaultServiceRequestJSON,
					},
				},
			},
			returnedBundle: defaultReturnedBundle,
		},
		{
			name:                   "unauthorised - fails",
			serviceRequestToCreate: defaultServiceRequest,
			ctx:                    context.Background(),
			expectError:            errors.New("not authenticated"),
		},
		{
			name:                   "non-local requester - fails",
			serviceRequestToCreate: defaultServiceRequest,
			ctx:                    auth.WithPrincipal(context.Background(), *auth.TestPrincipal2),
			expectError:            errors.New("Only the local care organization can create a ServiceRequest"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a ServiceRequest
			var serviceRequestToCreate = deep.Copy(defaultServiceRequest)
			if !deep.Equal(tt.serviceRequestToCreate, fhir.ServiceRequest{}) {
				serviceRequestToCreate = tt.serviceRequestToCreate
			}

			serviceRequestBytes, _ := json.Marshal(serviceRequestToCreate)
			fhirRequest := FHIRHandlerRequest{
				ResourcePath: "ServiceRequest",
				ResourceData: serviceRequestBytes,
				HttpMethod:   "POST",
				HttpHeaders: map[string][]string{
					"If-None-Exist": {"ifnoneexist"},
				},
			}

			tx := coolfhir.Transaction()

			mockFHIRClient := mock.NewMockClient(ctrl)
			fhirBaseUrl, _ := url.Parse("http://example.com/fhir")
			service := &Service{
				profile:    profile.Test(),
				fhirClient: mockFHIRClient,
				fhirURL:    fhirBaseUrl,
			}

			ctx := auth.WithPrincipal(context.Background(), *auth.TestPrincipal1)
			if tt.ctx != nil {
				ctx = tt.ctx
			}

			result, err := service.handleCreateServiceRequest(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "ServiceRequest/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
				*(result.(*[]byte)) = tt.createdServiceRequestBundle.Entry[0].Resource
				return nil
			}).AnyTimes()

			var returnedBundle = defaultReturnedBundle
			if tt.returnedBundle != nil {
				returnedBundle = tt.returnedBundle
			}
			require.Len(t, tx.Entry, len(returnedBundle.Entry))

			if tt.expectError == nil {
				t.Run("verify ServiceRequest", func(t *testing.T) {
					serviceRequestEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("ServiceRequest"))
					var serviceRequest fhir.ServiceRequest
					_ = json.Unmarshal(serviceRequestEntry.Resource, &serviceRequest)
					require.Equal(t, serviceRequestToCreate, serviceRequest)
				})
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, "ServiceRequest/1", *response.Response.Location)
			require.Equal(t, "201 Created", response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
