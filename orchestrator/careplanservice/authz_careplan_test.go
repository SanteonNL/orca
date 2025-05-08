package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestCarePlanAuthzPolicy(t *testing.T) {
	carePlan := fhir.CarePlan{
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
							Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
						},
					},
				},
			},
		}),
	}
	t.Run("read", func(t *testing.T) {
		policy := ReadCarePlanAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[fhir.CarePlan]{
			{
				name:      "allow (in CareTeam)",
				policy:    policy,
				resource:  carePlan,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (not in CareTeam)",
				policy:    policy,
				resource:  carePlan,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
}
