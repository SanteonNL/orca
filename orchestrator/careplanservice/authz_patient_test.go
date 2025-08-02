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

func TestPatientAuthzPolicy(t *testing.T) {
	patient := fhir.Patient{
		Id: to.Ptr("p1"),
	}
	patientWithCreator := patient
	patientWithCreator.Extension = TestCreatorExtension
	fhirClient := &test.StubFHIRClient{
		Resources: []any{
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
									Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
								},
							},
						},
					},
				}),
			},
		},
	}

	t.Run("create", func(t *testing.T) {
		policy := CreatePatientAuthzPolicy(profile.Test())
		testPolicies(t, []AuthzPolicyTest[*fhir.Patient]{
			{
				name:      "allow (is local organization)",
				policy:    policy,
				resource:  &patient,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (not local organization)",
				policy:    policy,
				resource:  &patient,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("read", func(t *testing.T) {
		policy := ReadPatientAuthzPolicy(FHIRClientFactoryFor(fhirClient))
		testPolicies(t, []AuthzPolicyTest[*fhir.Patient]{
			{
				name:      "allow (in CareTeam)",
				policy:    policy,
				resource:  &patient,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &patientWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (not in CareTeam)",
				policy:    policy,
				resource:  &patient,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("update", func(t *testing.T) {
		policy := UpdatePatientAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[*fhir.Patient]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &patientWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (principal isn't the creator of the Patient)",
				policy:    policy,
				resource:  &patientWithCreator,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
}
