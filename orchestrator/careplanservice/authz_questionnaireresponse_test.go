package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestQuestionnaireResponseAuthzPolicy(t *testing.T) {
	questionnaireResponse := fhir.QuestionnaireResponse{
		Id: to.Ptr("qr1"),
		Subject: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	questionnaireResponseWithCreator := questionnaireResponse
	questionnaireResponseWithCreator.Extension = TestCreatorExtension
	fhirClient := &test.StubFHIRClient{
		Resources: []any{
			fhir.Task{
				Id: to.Ptr("task1"),
				Requester: &fhir.Reference{
					Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
				},
				Output: []fhir.TaskOutput{
					{
						ValueReference: &fhir.Reference{
							Reference: to.Ptr("QuestionnaireResponse/qr1"),
						},
					},
				},
				Extension: TestCreatorExtension,
			},
		},
	}

	t.Run("create", func(t *testing.T) {
		policy := CreateQuestionnaireResponseAuthzPolicy(profile.Test())
		testPolicies(t, []AuthzPolicyTest[*fhir.QuestionnaireResponse]{
			{
				name:      "allow (is local organization)",
				policy:    policy,
				resource:  &questionnaireResponse,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (not local organization)",
				policy:    policy,
				resource:  &questionnaireResponse,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("read", func(t *testing.T) {
		policy := ReadQuestionnaireResponseAuthzPolicy(fhirClient)
		testPolicies(t, []AuthzPolicyTest[*fhir.QuestionnaireResponse]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &questionnaireResponseWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (principal has access to related Task)",
				policy:    policy,
				resource:  &questionnaireResponse,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (principal doesn't have access to related Task and not creator)",
				policy:    policy,
				resource:  &questionnaireResponseWithCreator,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("update", func(t *testing.T) {
		policy := UpdateQuestionnaireResponseAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[*fhir.QuestionnaireResponse]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &questionnaireResponseWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (principal doesn't have access to related Task and not creator)",
				policy:    policy,
				resource:  &questionnaireResponseWithCreator,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})

	t.Run("delete", func(t *testing.T) {
		policy := DeleteQuestionnaireResponseAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[*fhir.QuestionnaireResponse]{
			{
				name:      "allow (anyone can delete)",
				policy:    policy,
				resource:  &questionnaireResponse,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
		})
	})
}
