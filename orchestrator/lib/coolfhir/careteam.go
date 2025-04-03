package coolfhir

import (
	"context"
	"encoding/json"
	"fmt"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func resolveMember(ctx context.Context, client fhirclient.Client, member fhir.Reference) (fhir.Reference, error) {
	for {
		if member.Type != nil || member.Reference == nil || !IsLiteralReference(&member) {
			return member, nil
		}

		var resource RawResource

		err := client.ReadWithContext(ctx, *member.Reference, &resource)
		if err != nil {
			return member, nil
		}

		switch resource.Type {
		case "Organization":
			var organization fhir.Organization

			if err := json.Unmarshal(resource.Raw, &organization); err != nil {
				return member, fmt.Errorf("failed to unmarshal Organization: %w", err)
			}

			member = fhir.Reference{
				Type:       to.Ptr("Organization"),
				Reference:  nil,
				Identifier: &organization.Identifier[0],
			}
		case "PractitionerRole":
			var role fhir.PractitionerRole

			if err := json.Unmarshal(resource.Raw, &role); err != nil {
				return member, fmt.Errorf("failed to unmarshal PractitionerRole: %w", err)
			}

			member = *role.Organization
		case "Patient":
			return member, nil
		default:
			return member, fmt.Errorf("invalid member type: %s", resource.Type)
		}
	}
}

func ResolveParticipants(ctx context.Context, client fhirclient.Client, careTeam *fhir.CareTeam) error {
	for i, participant := range careTeam.Participant {
		if participant.Member == nil {
			continue
		}

		member, err := resolveMember(ctx, client, *participant.Member)
		if err != nil {
			return err
		}

		careTeam.Participant[i].Member = &member
	}

	return nil
}
