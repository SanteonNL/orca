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
	}

	serviceRequestWithID := deep.Copy(defaultServiceRequest)
	serviceRequestWithID.Id = to.Ptr("existing-servicerequest-id")
	serviceRequestWithIDJSON, _ := json.Marshal(serviceRequestWithID)

	returnedBundleForUpdate := &fhir.Bundle{
		Entry: []fhir.BundleEntry{
			{
				Response: &fhir.BundleEntryResponse{
					Location: to.Ptr("ServiceRequest/existing-servicerequest-id"),
					Status:   "200 OK",
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

	tests := []struct {
		name                        string
		serviceRequestToCreate      fhir.ServiceRequest
		createdServiceRequest       fhir.ServiceRequest
		createdServiceRequestBundle *fhir.Bundle
		returnedBundle              *fhir.Bundle
		expectError                 error
		principal                   *auth.Principal
		expectedMethod              string
		expectedURL                 string
	}{
		{
			name: "happy flow",
			createdServiceRequestBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: []byte(`{"status":"active","intent":"order","subject":{"identifier":{"system":"http://fhir.nl/fhir/NamingSystem/bsn","value":"1333333337"}}}`),
					},
				},
			},
		},
		{
			name:        "error: requester is not a local organization",
			principal:   auth.TestPrincipal2,
			expectError: errors.New("Only the local care organization can create a ServiceRequest"),
		},
		{
			name:                   "serviceRequest with existing ID - update",
			serviceRequestToCreate: serviceRequestWithID,
			createdServiceRequestBundle: &fhir.Bundle{
				Entry: []fhir.BundleEntry{
					{
						Resource: serviceRequestWithIDJSON,
					},
				},
			},
			returnedBundle: returnedBundleForUpdate,
			expectedMethod: "PUT",
			expectedURL:    "ServiceRequest/existing-servicerequest-id",
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
				Principal: auth.TestPrincipal1,
				LocalIdentity: to.Ptr(fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("1"),
				}),
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
			if tt.principal != nil {
				fhirRequest.Principal = tt.principal
			}

			if tt.expectedMethod == "PUT" {
				fhirRequest.HttpMethod = "PUT"
			}
			result, err := service.handleCreateServiceRequest(ctx, fhirRequest, tx)

			if tt.expectError != nil {
				require.EqualError(t, err, tt.expectError.Error())
				return
			}
			require.NoError(t, err)

			// For serviceRequest with ID, expect a different location path
			expectedLocation := "ServiceRequest/1"
			if serviceRequestToCreate.Id != nil {
				expectedLocation = "ServiceRequest/" + *serviceRequestToCreate.Id
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), expectedLocation, gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdServiceRequestBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			} else {
				mockFHIRClient.EXPECT().ReadWithContext(gomock.Any(), "ServiceRequest/1", gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, path string, result interface{}, option ...fhirclient.Option) error {
					*(result.(*[]byte)) = tt.createdServiceRequestBundle.Entry[0].Resource
					return nil
				}).AnyTimes()
			}

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

				// Verify the request method and URL for the serviceRequest entry
				if tt.expectedMethod != "" {
					serviceRequestEntry := coolfhir.FirstBundleEntry((*fhir.Bundle)(tx), coolfhir.EntryIsOfType("ServiceRequest"))
					require.Equal(t, tt.expectedMethod, serviceRequestEntry.Request.Method.String())
					require.Equal(t, tt.expectedURL, serviceRequestEntry.Request.Url)
				}
			}

			// Process result
			require.NotNil(t, result)
			response, notifications, err := result(returnedBundle)
			require.NoError(t, err)
			require.Equal(t, *returnedBundle.Entry[0].Response.Location, *response.Response.Location)
			require.Equal(t, returnedBundle.Entry[0].Response.Status, response.Response.Status)
			require.Len(t, notifications, 1)
		})
	}
}
