package policy_test

import data.policy

mock_input(principal, participants, resource, roles) := {
	"resource": resource,
	"roles": roles,
	"method": "GET", "principal": {
		"system": "http://fhir.nl/fhir/NamingSystem/ura",
		"value": principal,
	},
	"careplans": [{
		"resourceType": "CarePlan",
		"id": "cps-careplan-01",
		"status": "active",
		"intent": "order",
		"careTeam": [{
			"type": "CareTeam",
			"reference": "#cps-careteam-01",
		}],
		"contained": [{
			"resourceType": "CareTeam",
			"id": "cps-careteam-01",
			"participant": [
			{
				"member": {
					"type": "Organization",
					"identifier": {
						"system": "http://fhir.nl/fhir/NamingSystem/ura",
						"value": id,
					},
				},
				"period": {"start": "2024-08-27"},
			} |
				some id in participants
			],
		}],
	}],
}

test_allowed if {
	policy.allow with input as mock_input("11111111", ["11111111", "22222222"], {}, [])
}

test_allowed_with_tag if {
	policy.allow with input as mock_input(
		"11111111", ["11111111", "22222222"], {"meta": {"tags": [{
			"system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
			"value": "MH",
		}]}},
		["30.076"],
	)
}

test_tag_not_allowed if {
	not policy.allow with input as mock_input(
		"11111111", ["11111111", "22222222"], {"meta": {"tags": [{
			"system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
			"value": "MH",
		}]}},
		["01.045"],
	)
}

test_not_in_careteam if {
	not policy.allow with input as mock_input("11111111", ["22222222"], {}, [])
}

test_no_participants if {
	not policy.allow with input as mock_input("11111111", [], {}, [])
}

test_questionnaire if {
	policy.allow with input as {"method": "GET", "resource_type": "Questionnaire"}
}

test_post_denied if {
	not policy.allow with input as {"method": "POST"}
}
