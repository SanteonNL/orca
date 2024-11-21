package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/deep"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"reflect"
	"testing"
)

type GetResourceTestStruct[T any] struct {
	ctx                    context.Context
	mockClient             *mock.MockClient
	name                   string
	id                     string
	resourceType           string
	returnedResource       *T
	errorFromRead          error
	returnedCarePlanBundle *fhir.Bundle
	errorFromCarePlanRead  error
	// For resources that require a Task Search to validate
	returnedTaskBundle      *fhir.Bundle
	errorFromTaskBundleRead error
	// For resources that are part of a task
	returnedTaskId    string
	returnedTask      *fhir.Task
	errorFromTaskRead error
	// For resources that require a Patient Search to validate
	returnedPatientBundle      *fhir.Bundle
	errorFromPatientBundleRead error
	expectedError              error
}

func testHelperHandleGetResource[T any](t *testing.T, params GetResourceTestStruct[T], handler func(ctx context.Context, id string, headers *fhirclient.Headers) (*T, error)) {
	t.Run(fmt.Sprintf("Test %s: %s", params.resourceType, params.name), func(t *testing.T) {
		if params.returnedCarePlanBundle != nil || params.errorFromCarePlanRead != nil {
			params.mockClient.EXPECT().Read("CarePlan", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedCarePlanBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedCarePlanBundle))
				}
				return params.errorFromCarePlanRead
			})
		}
		if params.returnedTaskBundle != nil || params.errorFromTaskBundleRead != nil {
			params.mockClient.EXPECT().Read("Task", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedTaskBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedTaskBundle))
				}
				return params.errorFromTaskBundleRead
			})
		}
		if params.returnedTask != nil || params.errorFromTaskRead != nil {
			params.mockClient.EXPECT().Read("Task/"+params.returnedTaskId, gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedTask != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedTask))
				}
				return params.errorFromTaskRead
			})
		}
		if params.returnedPatientBundle != nil || params.errorFromPatientBundleRead != nil {
			params.mockClient.EXPECT().Read("Patient", gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedPatientBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedPatientBundle))
				}
				return params.errorFromPatientBundleRead
			})
		}
		if (params.returnedResource != nil || params.errorFromRead != nil) && params.resourceType != "CarePlan" && params.resourceType != "Patient" {
			params.mockClient.EXPECT().Read(params.resourceType+"/"+params.id, gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result interface{}, option ...fhirclient.Option) error {
				if params.returnedResource != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedResource))
				}
				return params.errorFromRead
			})
		}

		got, err := handler(params.ctx, params.id, &fhirclient.Headers{})
		if params.expectedError != nil {
			// assert that we got the expected error
			require.Error(t, err)
			require.Equal(t, params.expectedError, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, params.returnedResource, got)
		}
	})
}

type HandleUpdateMetaBasedResourceTestStruct[T any] struct {
	ctx              context.Context
	mockClient       *mock.MockClient
	name             string
	id               string
	resourceType     string
	expectedError    error
	existingResource *T
	errorFromRead    error
	request          []func(*T)
}

func testHelperHandleUpdateMetaBasedResource[T any](t *testing.T, params HandleUpdateMetaBasedResourceTestStruct[T], handler func(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error)) {
	t.Run(fmt.Sprintf("Test %s: %s", params.resourceType, params.name), func(t *testing.T) {
		updateRequest := func(fn ...func(*T)) FHIRHandlerRequest {
			updatedResource := deep.Copy(params.existingResource)
			for _, f := range fn {
				f(updatedResource)
			}
			updatedResourceData, _ := json.Marshal(updatedResource)
			// Get ID from updatedResourceData through unmarshalling
			type ResourceID struct {
				Id *string `json:"id,omitempty"`
			}
			var resourceId ResourceID
			_ = json.Unmarshal(updatedResourceData, &resourceId)

			requestUrl, _ := url.Parse(fmt.Sprintf("%s/%s", params.resourceType, params.id))
			return FHIRHandlerRequest{
				ResourceData: updatedResourceData,
				ResourcePath: requestUrl.Path,
				ResourceId:   *resourceId.Id,
				RequestUrl:   requestUrl,
				HttpMethod:   "PUT",
			}
		}

		params.mockClient.EXPECT().Read(fmt.Sprintf("%s/%s", params.resourceType, params.id), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, result *T, option ...fhirclient.Option) error {
			if params.existingResource != nil {
				reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.existingResource))
			}
			return params.errorFromRead
		}).AnyTimes()

		request := updateRequest(params.request...)
		tx := coolfhir.Transaction()
		result, err := handler(params.ctx, request, tx)
		if params.expectedError != nil {
			require.Error(t, err)
			require.Equal(t, params.expectedError, err)
		} else {
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, 1, len(tx.Entry))
			require.Equal(t, fmt.Sprintf("%s/%s", params.resourceType, params.id), tx.Entry[0].Request.Url)
			require.Equal(t, fhir.HTTPVerbPUT, tx.Entry[0].Request.Method)
		}
	})
}

func getOrgRef(principal auth.Principal) string {
	return fmt.Sprintf("%s/%s", *principal.Organization.Identifier[0].System, *principal.Organization.Identifier[0].Value)
}
