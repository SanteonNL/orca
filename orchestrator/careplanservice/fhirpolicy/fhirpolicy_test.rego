package fhirpolicy_test

import data.fhirpolicy

mock_input(principal, participants) := {
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
	fhirpolicy.allow with input as mock_input("11111111", ["11111111", "22222222"])
}

test_not_in_careteam if {
	not fhirpolicy.allow with input as mock_input("11111111", ["22222222"])
}

test_no_careplans if {
	not fhirpolicy.allow with input as mock_input("11111111", [])
}

test_questionnaire if {
	fhirpolicy.allow with input as {"method": "GET", "resource_type": "Questionnaire"}
}

test_post_denied if {
	not fhirpolicy.allow with input as {"method": "POST"}
}
