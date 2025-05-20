package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestCreateQuestionnaireAuthzPolicy(t *testing.T) {
	questionnaire := fhir.Questionnaire{}
	policy := CreateQuestionnaireAuthzPolicy()
	testPolicies(t, []AuthzPolicyTest[*fhir.Questionnaire]{
		{
			name:      "allow (anyone)",
			policy:    policy,
			resource:  &questionnaire,
			principal: auth.TestPrincipal2,
			wantAllow: true,
		},
	})
}

func TestUpdateQuestionnaireAuthzPolicy(t *testing.T) {
	questionnaire := fhir.Questionnaire{}
	policy := UpdateQuestionnaireAuthzPolicy()
	testPolicies(t, []AuthzPolicyTest[*fhir.Questionnaire]{
		{
			name:      "allow (anyone)",
			policy:    policy,
			resource:  &questionnaire,
			principal: auth.TestPrincipal2,
			wantAllow: true,
		},
	})
}

func TestReadQuestionnaireAuthzPolicy(t *testing.T) {
	questionnaire := fhir.Questionnaire{}
	policy := ReadQuestionnaireAuthzPolicy()
	testPolicies(t, []AuthzPolicyTest[*fhir.Questionnaire]{
		{
			name:      "allow (anyone)",
			policy:    policy,
			resource:  &questionnaire,
			principal: auth.TestPrincipal2,
			wantAllow: true,
		},
	})
}

func TestDeleteQuestionnaireAuthzPolicy(t *testing.T) {
	questionnaire := fhir.Questionnaire{}
	policy := DeleteQuestionnaireAuthzPolicy()
	testPolicies(t, []AuthzPolicyTest[*fhir.Questionnaire]{
		{
			name:      "allow (anyone can delete)",
			policy:    policy,
			resource:  &questionnaire,
			principal: auth.TestPrincipal1,
			wantAllow: true,
		},
	})
}
