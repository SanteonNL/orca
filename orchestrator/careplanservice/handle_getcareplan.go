package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
)

func ReadCarePlanAuthzPolicy() Policy[fhir.CarePlan] {
	return CareTeamMemberPolicy[fhir.CarePlan]{}
}

// handleReadCarePlan fetches the requested CarePlan and validates if the requester has access to the resource (is a participant of one of the CareTeams of the care plan)
// if the requester is valid, return the CarePlan, else return an error
func (s *Service) handleReadCarePlan(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting CarePlan with ID: %s", request.ResourceId)
	var carePlan fhir.CarePlan

	// fetch CarePlan, validate requester is participant of CareTeam
	err := s.fhirClient.ReadWithContext(ctx, "CarePlan/"+request.ResourceId, &carePlan, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}
	hasAccess, err := ReadCarePlanAuthzPolicy().HasAccess(ctx, carePlan, *request.Principal)
	if err != nil {
		return nil, err
	}
	if !hasAccess {
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to CarePlan",
			StatusCode: http.StatusForbidden,
		}
	}

	carePlanRaw, err := json.Marshal(carePlan)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: carePlanRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        carePlan.Id,
		Type:      to.Ptr("CarePlan"),
		Reference: to.Ptr("CarePlan/" + *carePlan.Id),
	}, &fhir.Reference{
		Identifier: &request.Principal.Organization.Identifier[0],
		Type:       to.Ptr("Organization"),
	})
	tx.Create(auditEvent)

	return func(txResult *fhir.Bundle) ([]*fhir.BundleEntry, []any, error) {
		// We do not want to notify subscribers for a get
		return []*fhir.BundleEntry{&bundleEntry}, []any{}, nil
	}, nil
}
