package policy

roles_to_tags := {
	# Uroloog (01.045)
	"01.045": [
		"ETH", # substance abuse information sensitivity
		"GDIS", # genetic disease information sensitivity
		"SDV", # sexual assault, abuse, or domestic violence information sensitivity
		"SEX", # sexuality and reproductive health information sensitivity
		"STD", # sexually transmitted disease information sensitivity
	],
	# Verpl. spec. algemene gezondheidszorg (30.076)
	"30.076": [
		"ETH", # substance abuse information sensitivity
		"SDV", # sexual assault, abuse, or domestic violence information sensitivity
		"BH", # behavioral health information sensitivity
		"MH", # mental health information sensitivity
	],
}

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

extract_tags(resource) := [tag.value |
	some tag in resource.meta.tags
	tag.system == "http://terminology.hl7.org/CodeSystem/v3-ActCode"
]

contains_tags if {
	input.resource != null
	input.resource.meta != null
	input.resource.meta.tags != null
}

is_careteam_participant if {
	some care_plan in input.careplans
	care_teams := extract_care_teams(care_plan)
	some care_team in care_teams
	some participant in care_team.participant
	identifier_matches(participant.member.identifier, input.principal)
}

is_get_post if {
	input.method in ["GET", "POST"]
}

default allow := false

allow if {
	input.method == "GET"
	input.resource.resourceType == "Questionnaire"
}

allow if {
	input.method == "GET"
	input.resource.resourceType == "QuestionnaireResponse"
}

allow if {
	is_get_post
	is_careteam_participant
	not contains_tags
}

allow if {
	is_get_post
	is_careteam_participant
	contains_tags

	allowed_tags := [tag |
		some role in input.roles
		role_tags := roles_to_tags[role]
		some tag in role_tags
	]
	every tag in extract_tags(input.resource) {
		tag in allowed_tags
	}
}
