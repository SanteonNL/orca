package careplanservice

import (
	"context"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/assert"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"go.uber.org/mock/gomock"
	"net/url"
	"testing"
)

type AuthzPolicyTest[T any] struct {
	name       string
	principal  *auth.Principal
	wantAllow  bool
	wantErr    error
	skipReason string
	resource   T
	policy     Policy[T]
}

func testPolicies[T any](t *testing.T, tests []AuthzPolicyTest[T]) {
	t.Helper()
	for _, tt := range tests {
		testPolicy(t, tt)
	}
}

func testPolicy[T any](t *testing.T, tt AuthzPolicyTest[T]) {
	t.Helper()
	t.Run(tt.name, func(t *testing.T) {
		if tt.skipReason != "" {
			t.Skip(tt.skipReason)
		}
		hasAccess, err := tt.policy.HasAccess(context.Background(), tt.resource, *tt.principal)
		assert.Equal(t, tt.wantAllow, hasAccess.Allowed)
		if tt.wantErr != nil {
			assert.EqualError(t, err, tt.wantErr.Error())
		} else {
			assert.NoError(t, err)
		}
	})
}

func TestRelatedResourcePolicy_HasAccess(t *testing.T) {
	t.Skip()
	t.Run("pagination of related resources", func(t *testing.T) {
		// Use CarePlan as "related resource" that gives access.
		// 3 CarePlans, the last one gives access.
		carePlan1 := fhir.CarePlan{
			Id: to.Ptr("1"),
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1"),
				},
			},
		}
		carePlan3 := fhir.CarePlan{
			Id: to.Ptr("3"),
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1"),
				},
			},
		}
		carePlan2 := fhir.CarePlan{
			Id: to.Ptr("2"),
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1"),
				},
			},
		}
		fhirClient := &test.StubFHIRClient{
			Resources: []any{carePlan1, carePlan2, carePlan3},
		}
		ctrl := gomock.NewController(t)
		relatedPolicy := NewMockPolicy[fhir.CarePlan](ctrl)
		relatedPolicy.EXPECT().HasAccess(gomock.Any(), carePlan1, gomock.Any()).Return(&PolicyDecision{Allowed: false}, nil)
		relatedPolicy.EXPECT().HasAccess(gomock.Any(), carePlan2, gomock.Any()).Return(&PolicyDecision{Allowed: false}, nil)
		relatedPolicy.EXPECT().HasAccess(gomock.Any(), carePlan3, gomock.Any()).Return(&PolicyDecision{Allowed: true}, nil)

		result, err := RelatedResourcePolicy[fhir.Patient, fhir.CarePlan]{
			fhirClient:            fhirClient,
			relatedResourcePolicy: relatedPolicy,
			relatedResourceSearchParams: func(_ context.Context, _ fhir.Patient) (resourceType string, searchParams url.Values) {
				return "CarePlan", url.Values{
					"subject": []string{"http://fhir.nl/fhir/NamingSystem/bsn|1"},
					"_count":  []string{"1"}, // 1-result pages for the test
				}
			},
		}.HasAccess(context.Background(), fhir.Patient{}, *auth.TestPrincipal1)

		assert.NoError(t, err)
		assert.True(t, result.Allowed)
	})
}

var TestCreatorExtension = []fhir.Extension{{
	Url: CreatorExtensionURL,
	ValueReference: &fhir.Reference{
		Type: to.Ptr("Organization"),
		Identifier: &fhir.Identifier{
			System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
			Value:  to.Ptr("1"),
		},
	}},
}

func TestSetCreatorExtensionOnResource(t *testing.T) {
	tests := []struct {
		name       string
		resource   fhir.Task
		identifier *fhir.Identifier
		want       func(t *testing.T, resource fhir.Task)
	}{
		{
			name: "add creator extension to resource without extensions",
			resource: fhir.Task{
				Id: to.Ptr("task-1"),
			},
			identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("12345"),
			},
			want: func(t *testing.T, resource fhir.Task) {
				assert.Len(t, resource.Extension, 1)
				assert.Equal(t, CreatorExtensionURL, resource.Extension[0].Url)
				assert.NotNil(t, resource.Extension[0].ValueReference)
				assert.Equal(t, "Organization", *resource.Extension[0].ValueReference.Type)
				assert.Equal(t, "http://fhir.nl/fhir/NamingSystem/ura", *resource.Extension[0].ValueReference.Identifier.System)
				assert.Equal(t, "12345", *resource.Extension[0].ValueReference.Identifier.Value)
			},
		},
		{
			name: "add creator extension to resource with other extensions",
			resource: fhir.Task{
				Id: to.Ptr("task-1"),
				Extension: []fhir.Extension{
					{
						Url:          "http://example.org/other-extension",
						ValueBoolean: to.Ptr(true),
					},
				},
			},
			identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("12345"),
			},
			want: func(t *testing.T, resource fhir.Task) {
				assert.Len(t, resource.Extension, 2)
				assert.Equal(t, "http://example.org/other-extension", resource.Extension[0].Url)
				assert.Equal(t, CreatorExtensionURL, resource.Extension[1].Url)
				assert.NotNil(t, resource.Extension[1].ValueReference)
			},
		},
		{
			name: "replace existing creator extension",
			resource: fhir.Task{
				Id: to.Ptr("task-1"),
				Extension: []fhir.Extension{
					{
						Url: CreatorExtensionURL,
						ValueReference: &fhir.Reference{
							Type: to.Ptr("Organization"),
							Identifier: &fhir.Identifier{
								System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
								Value:  to.Ptr("old-value"),
							},
						},
					},
				},
			},
			identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("new-value"),
			},
			want: func(t *testing.T, resource fhir.Task) {
				assert.Len(t, resource.Extension, 1)
				assert.Equal(t, CreatorExtensionURL, resource.Extension[0].Url)
				assert.NotNil(t, resource.Extension[0].ValueReference)
				assert.Equal(t, "new-value", *resource.Extension[0].ValueReference.Identifier.Value)
			},
		},
		{
			name: "remove multiple creator extensions and add new one",
			resource: fhir.Task{
				Id: to.Ptr("task-1"),
				Extension: []fhir.Extension{
					{
						Url: CreatorExtensionURL,
						ValueReference: &fhir.Reference{
							Identifier: &fhir.Identifier{
								Value: to.Ptr("first"),
							},
						},
					},
					{
						Url: "http://example.org/other-extension",
					},
					{
						Url: CreatorExtensionURL,
						ValueReference: &fhir.Reference{
							Identifier: &fhir.Identifier{
								Value: to.Ptr("second"),
							},
						},
					},
				},
			},
			identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/ura"),
				Value:  to.Ptr("new"),
			},
			want: func(t *testing.T, resource fhir.Task) {
				assert.Len(t, resource.Extension, 2)
				assert.Equal(t, "http://example.org/other-extension", resource.Extension[0].Url)
				assert.Equal(t, CreatorExtensionURL, resource.Extension[1].Url)
				assert.Equal(t, "new", *resource.Extension[1].ValueReference.Identifier.Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetCreatorExtensionOnResource(&tt.resource, tt.identifier)
			tt.want(t, tt.resource)
		})
	}
}
