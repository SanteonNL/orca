package careplanservice

import (
	"github.com/SanteonNL/orca/orchestrator/cmd/profile"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"testing"
)

func TestServiceRequestAuthzPolicy(t *testing.T) {
	serviceRequest := fhir.ServiceRequest{
		Id:     to.Ptr("sr1"),
		Status: fhir.RequestStatusActive,
		Intent: fhir.RequestIntentOrder,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	serviceRequestWithCreator := serviceRequest
	serviceRequestWithCreator.Extension = TestCreatorExtension
	fhirClient := &test.StubFHIRClient{
		Resources: []any{
			fhir.Task{
				Id: to.Ptr("task1"),
				Requester: &fhir.Reference{
					Identifier: &auth.TestPrincipal1.Organization.Identifier[0],
				},
				Focus: &fhir.Reference{
					Reference: to.Ptr("ServiceRequest/sr1"),
				},
			},
		},
	}

	t.Run("create", func(t *testing.T) {
		policy := CreateServiceRequestAuthzPolicy(profile.Test())
		testPolicies(t, []AuthzPolicyTest[*fhir.ServiceRequest]{
			{
				name:      "allow (is local organization)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (not local organization)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("read", func(t *testing.T) {
		policy := ReadServiceRequestAuthzPolicy(fhirClient)
		testPolicies(t, []AuthzPolicyTest[*fhir.ServiceRequest]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &serviceRequestWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (principal has access to related Task)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (principal doesn't have access to related Task)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})
	t.Run("update", func(t *testing.T) {
		policy := ReadServiceRequestAuthzPolicy(fhirClient)
		testPolicies(t, []AuthzPolicyTest[*fhir.ServiceRequest]{
			{
				name:      "allow (is creator)",
				policy:    policy,
				resource:  &serviceRequestWithCreator,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "allow (principal has access to related Task)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
			{
				name:      "disallow (principal doesn't have access to related Task)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal2,
				wantAllow: false,
			},
		})
	})

	t.Run("delete", func(t *testing.T) {
		policy := DeleteServiceRequestAuthzPolicy()
		testPolicies(t, []AuthzPolicyTest[*fhir.ServiceRequest]{
			{
				name:      "allow (anyone can delete)",
				policy:    policy,
				resource:  &serviceRequest,
				principal: auth.TestPrincipal1,
				wantAllow: true,
			},
		})
	})

}
