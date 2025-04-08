package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/careplancontributor/mock"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
)

var testPrincipal = auth.Principal{
	Organization: fhir.Organization{
		Id:   to.Ptr("1"),
		Name: to.Ptr("Applesauce B.V."),
		Address: []fhir.Address{{
			City: to.Ptr("Meppel"),
		}},
		Identifier: []fhir.Identifier{{
			Id:     to.Ptr("2"),
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("22222222"),
		}},
	},
}

type input map[string]any

func TestAllow(t *testing.T) {
	tests := map[string]struct {
		policy        string
		expectedError error
	}{
		"request is allowed": {
			policy: "allow := true",
		},
		"request is not allowed": {
			policy:        "allow := false",
			expectedError: ErrAccessDenied,
		},
		"evaluation failed": {
			policy:        "",
			expectedError: errors.New("policy evaluation failed"),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.TODO()
			agent, err := NewAgent(ctx, RegoModule{
				Package: "example",
				Source:  fmt.Sprintf("package example\n%s", tt.policy),
			}, nil)
			require.NoError(t, err)

			err = agent.Allow(ctx, &Context{})

			if tt.expectedError != nil {
				require.Equal(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSubjectToParams(t *testing.T) {
	tests := map[string]struct {
		reference fhir.Reference
		expected  url.Values
	}{
		"type and reference": {
			reference: fhir.Reference{
				Type:      to.Ptr("Patient"),
				Reference: to.Ptr("Patient/minimal-enrollment-Patient"),
			},
			expected: url.Values{"subject": []string{"Patient/minimal-enrollment-Patient"}},
		},
		"logical reference": {
			reference: fhir.Reference{
				Id:   to.Ptr("11111111"),
				Type: to.Ptr("Patient"),
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
					Value:  to.Ptr("11111111"),
				},
			},
			expected: url.Values{"patient:Patient.identifier": []string{"http://fhir.nl/fhir/NamingSystem/ura|11111111"}},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			params, err := subjectToParams(&tt.reference)

			require.NoError(t, err)
			require.Equal(t, tt.expected, params)
		})
	}
}

func TestPreflight(t *testing.T) {
	tests := map[string]struct {
		id             string
		resourceType   string
		request        func() *http.Request
		expectedResult *Preflight
		expectedErr    string
	}{
		"happy path": {
			id:           "minimal-enrollment-Patient",
			resourceType: "Patient",
			expectedResult: &Preflight{
				ResourceType: "Patient",
				ResourceId:   "minimal-enrollment-Patient",
				Method:       http.MethodGet,
				Principal:    testPrincipal.Organization.Identifier[0],
				Roles:        []string{"001.001.001"},
				Query:        url.Values{"x": []string{"y"}},
			},
			request: func() *http.Request {
				request := httptest.NewRequest(http.MethodGet, "/Patient/minimal-enrollment-Patient?x=y", nil)
				request.Header.Set("Orca-Auth-Roles", "001.001.001")
				return request.WithContext(auth.WithPrincipal(request.Context(), testPrincipal))
			},
		},
		"not authenticated": {
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/", nil)
			},
			expectedErr: "failed to extract principal from context: not authenticated",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			preflight, err := Agent{}.Preflight(tt.resourceType, tt.id, tt.request())

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, preflight)
			} else {
				require.ErrorContains(t, err, tt.expectedErr)
				require.Nil(t, preflight)
			}
		})
	}
}

func TestPrepareContext(t *testing.T) {
	testTask := fhir.Task{For: &fhir.Reference{Id: to.Ptr("1")}}
	testCarePlan := fhir.CarePlan{Id: to.Ptr("2")}
	testCarePlanBody, err := json.Marshal(testCarePlan)
	require.NoError(t, err)

	tests := map[string]struct {
		resource       any
		resourceType   string
		expectedErr    string
		expectedResult *Context
		setup          func(ctx context.Context, mock *mock.MockClient)
	}{
		"careplans are fetched correctly": {
			resourceType: "Task",
			resource:     testTask,
			setup: func(ctx context.Context, mock *mock.MockClient) {
				mock.EXPECT().
					SearchWithContext(ctx, "CarePlan", url.Values{
						"subject": []string{"Patient/1"},
					}, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ string, _ url.Values, target any, _ ...fhirclient.Option) error {
						*target.(*fhir.Bundle) = fhir.Bundle{
							Entry: []fhir.BundleEntry{
								{Resource: nil},
								{Resource: testCarePlanBody},
							},
						}
						return nil
					})
			},
			expectedResult: &Context{
				Principal: testPrincipal.Organization.Identifier[0],
				Method:    http.MethodGet,
				Resource:  testTask,
				CarePlans: []fhir.CarePlan{testCarePlan},
			},
		},
		"failed to search": {
			resourceType: "Task",
			resource:     testTask,
			setup: func(ctx context.Context, mock *mock.MockClient) {
				mock.EXPECT().
					SearchWithContext(ctx, "CarePlan", url.Values{
						"subject": []string{"Patient/1"},
					}, gomock.Any()).
					Return(errors.New("random error"))
			},
			expectedErr: "failed to search for careplans: random error",
		},
		"missing subject": {
			resourceType: "Task",
			resource:     fhir.Task{For: &fhir.Reference{}},
			expectedErr:  "invalid subject",
		},
		"unsupported resource": {
			resourceType: "Bazinga",
			expectedErr:  "failed to extract subject: unsupported resource type: Bazinga",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/Patient/minimal-enrollment-Patient?x=y", nil)
			request = request.WithContext(auth.WithPrincipal(request.Context(), testPrincipal))

			ctrl := gomock.NewController(t)
			client := mock.NewMockClient(ctrl)

			if tt.setup != nil {
				tt.setup(request.Context(), client)
			}

			a := Agent{client: client}

			preflight, err := a.Preflight(tt.resourceType, "", request)
			require.NoError(t, err)

			context, err := a.PrepareContext(request.Context(), NewSearchCache(), preflight, tt.resource)

			if tt.expectedErr == "" {
				require.NoError(t, err)
				require.Equal(t, tt.expectedResult, context)
			} else {
				require.ErrorContains(t, err, tt.expectedErr)
				require.Nil(t, context)
			}
		})
	}
}
