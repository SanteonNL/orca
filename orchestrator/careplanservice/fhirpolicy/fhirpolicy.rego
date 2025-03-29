package fhirpolicy

identifier_matches(left, right) if {
	left.system == right.system
	left.value == right.value
}

extract_care_teams(care_plan) := [resource |
	some care_team in care_plan.careTeam
	some resource in care_plan.contained
	resource.resourceType == "CareTeam"
	resource.id == trim_left(care_team.reference, "#")
]

default allow := false

allow if {
	input.method == "GET"
	input.resource_type == "Questionnaire"
}

allow if {
	input.method == "GET"

	some care_plan in input.careplans
	care_teams := extract_care_teams(care_plan)
	some care_team in care_teams
	some participant in care_team.participant
	identifier_matches(participant.member.identifier, input.principal)
}
