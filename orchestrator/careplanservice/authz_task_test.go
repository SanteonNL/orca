package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestReadTaskAuthzPolicy(t *testing.T) {
	taskWithRequesterAndOwner := fhir.Task{
		Id: to.Ptr("task1"),
		BasedOn: []fhir.Reference{
			{
				Type:      to.Ptr("CarePlan"),
				Reference: to.Ptr("CarePlan/cp1"),
			},
		},
		Requester: &fhir.Reference{
			Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
		},
		Owner: &fhir.Reference{
			Identifier: &auth.TestPrincipal2.Organization.Identifier[0],
		},
	}
	taskWithRequester := fhir.Task{
		Id: to.Ptr("task1"),
		BasedOn: []fhir.Reference{
			{
				Type:      to.Ptr("CarePlan"),
				Reference: to.Ptr("CarePlan/cp1"),
			},
		},
		Requester: &fhir.Reference{
			Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
		},
	}
	fhirClient := &test.StubFHIRClient{
		Resources: []any{
			fhir.CarePlan{
				Id: to.Ptr("cp1"),
				CareTeam: []fhir.Reference{
					{
						Type:      to.Ptr("CareTeam"),
						Reference: to.Ptr("#ct"),
					},
				},
				Contained: must.MarshalJSON([]fhir.CareTeam{
					{
						Id: to.Ptr("ct"),
						Participant: []fhir.CareTeamParticipant{
							{
								Member: &fhir.Reference{
									Type:       to.Ptr("Organization"),
									Identifier: &auth.TestPrincipal3.Organization.Identifier[0],
								},
							},
						},
					},
				}),
			},
		},
	}

	t.Run("read", func(t *testing.T) {
		policy := ReadTaskAuthzPolicy(FHIRClientFactoryFor(fhirClient))
		testPolicies(t, []AuthzPolicyTest[*fhir.Task]{
			{
				name:      "allow (principal is Task requester)",
				policy:    policy,
				resource:  &taskWithRequesterAndOwner,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (principal is Task requester)",
				policy:    policy,
				resource:  &taskWithRequesterAndOwner,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (principal is in CareTeam)",
				policy:    policy,
				resource:  &taskWithRequesterAndOwner,
				principal: auth.TestPrincipal3,
				wantAllow: true,
			},
			{
				name:      "disallow (principal is neither Task requester, nor owner, nor in CareTeam)",
				policy:    policy,
				resource:  &taskWithRequester,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
}
