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
