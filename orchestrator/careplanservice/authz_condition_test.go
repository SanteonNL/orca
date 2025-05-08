package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/must"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestConditionAuthzPolicy(t *testing.T) {
	condition := fhir.Condition{
		Id: to.Ptr("c1"),
		Subject: fhir.Reference{
			Type: to.Ptr("Patient"),
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	fhirClient := &test.StubFHIRClient{
		Resources: []any{
			fhir.Patient{
				Id: to.Ptr("p1"),
				Identifier: []fhir.Identifier{
					{
						System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
						Value:  to.Ptr("1333333337"),
					},
				},
			},
			fhir.CarePlan{
				Id: to.Ptr("cp1"),
				Subject: fhir.Reference{
					Type:      to.Ptr("Patient"),
					Reference: to.Ptr("Patient/p1"),
				},
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
									Identifier: &auth.TestPrincipal2.Organization.Identifier[0],
								},
							},
						},
					},
				}),
			},
		},
	}

	t.Run("create", func(t *testing.T) {
		policy := CreateConditionAuthzPolicy(profile.Test())
		testPolicies(t, []AuthzPolicyTest[fhir.Condition]{
			{
				name:      "allow (is local organization)",
				policy:    policy,
				resource:  condition,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:       "disallow (not local organization)",
				policy:     policy,
				resource:   condition,
				principal:  auth.TestPrincipal2,
				wantAllow:  false,
				skipReason: "'is creator' policy always returns true",
			},
		})
	})
	t.Run("read", func(t *testing.T) {
		policy := ReadConditionAuthzPolicy(fhirClient)
		testPolicies(t, []AuthzPolicyTest[fhir.Condition]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  condition,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (access to related Patient)",
				policy:    policy,
				resource:  condition,
				principal: auth.TestPrincipal2,
				wantAllow: true,
			},
			{
				name:       "disallow (no access to related Patient)",
				policy:     policy,
				resource:   condition,
				principal:  auth.TestPrincipal3,
				wantAllow:  false,
				skipReason: "'is creator' policy always returns true",
			},
		})
	})
	t.Run("update", func(t *testing.T) {
		policy := UpdateConditionAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[fhir.Condition]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  condition,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:       "disallow (principal isn't the creator of the Condition)",
				policy:     policy,
				resource:   condition,
				principal:  auth.TestPrincipal2,
				wantAllow:  false,
				skipReason: "'is creator' policy always returns true",
			},
		})
	})
}
