package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"reflect"
	"testing"
)

type TestHandleGetStruct[T any] struct {
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

type TestHandleSearchStruct[T any] struct {
	ctx                    context.Context
	mockClient             *mock.MockClient
	name                   string
	resourceType           string
	searchParams           url.Values
	returnedBundle         *fhir.Bundle
	errorFromSearch        error
	returnedCarePlanBundle *fhir.Bundle
	errorFromCarePlanRead  error
	expectedBundle         *fhir.Bundle
	expectedError          error
}

func testHelperHandleGetResource[T any](t *testing.T, params TestHandleGetStruct[T], handler func(ctx context.Context, id string, headers *fhirclient.Headers) (*T, error)) {
	t.Run(fmt.Sprintf("Test %s: %s", params.resourceType, params.name), func(t *testing.T) {
		if params.returnedCarePlanBundle != nil || params.errorFromCarePlanRead != nil {
			params.mockClient.EXPECT().Search("CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, searchParams url.Values, result interface{}, option ...fhirclient.Option) error {
				if params.returnedCarePlanBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedCarePlanBundle))
				}
				return params.errorFromCarePlanRead
			})
		}
		if params.returnedTaskBundle != nil || params.errorFromTaskBundleRead != nil {
			params.mockClient.EXPECT().Search("Task", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, searchParams url.Values, result interface{}, option ...fhirclient.Option) error {
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
			params.mockClient.EXPECT().Search("Patient", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, searchParams url.Values, result interface{}, option ...fhirclient.Option) error {
				if params.returnedPatientBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedPatientBundle))
				}
				return params.errorFromPatientBundleRead
			})
		}
		if (params.returnedResource != nil || params.errorFromRead != nil) && params.resourceType != "CarePlan" {
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

func testHelperHandleSearchResource[T any](t *testing.T, params TestHandleSearchStruct[T], handler func(ctx context.Context, searchParams url.Values, headers *fhirclient.Headers) (*fhir.Bundle, error)) {
	t.Run(fmt.Sprintf("Test %s: %s", params.resourceType, params.name), func(t *testing.T) {
		if params.returnedCarePlanBundle != nil || params.errorFromCarePlanRead != nil {
			params.mockClient.EXPECT().Search("CarePlan", gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(path string, searchParams url.Values, result interface{}, option ...fhirclient.Option) error {
				if params.returnedCarePlanBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedCarePlanBundle))
				}
				return params.errorFromCarePlanRead
			})
		}

		if params.returnedBundle != nil || params.errorFromSearch != nil {
			params.mockClient.EXPECT().Search(params.resourceType, gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(resourceType string, searchParams url.Values, result interface{}, option ...fhirclient.Option) error {
				if params.returnedBundle != nil {
					reflect.ValueOf(result).Elem().Set(reflect.ValueOf(*params.returnedBundle))
				}
				return params.errorFromSearch
			})
		}

		got, err := handler(params.ctx, params.searchParams, &fhirclient.Headers{})
		if params.expectedError != nil {
			// assert that we got the expected error
			require.Error(t, err)
			require.Equal(t, params.expectedError, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, params.expectedBundle, got)
		}
	})
}
