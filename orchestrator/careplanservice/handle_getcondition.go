package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/audit"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"net/url"
)

func ReadConditionAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Condition] {
	// TODO: Find out new auth requirements for condition
	return AnyMatchPolicy[fhir.Condition]{
		Policies: []Policy[fhir.Condition]{
			RelatedResourceSearchPolicy[fhir.Condition, fhir.Patient]{
				fhirClient:            fhirClient,
				relatedResourcePolicy: ReadPatientAuthzPolicy(fhirClient),
				relatedResourceSearchParams: func(ctx context.Context, resource fhir.Condition) (string, *url.Values) {
					if resource.Subject.Identifier == nil || resource.Subject.Identifier.System == nil || resource.Subject.Identifier.Value == nil {
						log.Ctx(ctx).Warn().Msg("Condition does not have Patient as subject, can't verify access")
						return "Patient", nil
					}
					return "Patient", &url.Values{
						"identifier": []string{fmt.Sprintf("%s|%s", *resource.Subject.Identifier.System, *resource.Subject.Identifier.Value)},
					}
				},
			},
			CreatorPolicy[fhir.Condition]{},
		},
	}
}

// handleReadCondition fetches the requested Condition and validates if the requester has access to the resource
// by checking if they have access to the Patient referenced in the Condition's subject
// if the requester is valid, return the Condition, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleReadCondition(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Condition with ID: %s", request.ResourceId)
	var condition fhir.Condition
	err := s.fhirClient.ReadWithContext(ctx, "Condition/"+request.ResourceId, &condition, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	hasAccess, err := ReadConditionAuthzPolicy(s.fhirClient).HasAccess(ctx, condition, *request.Principal)
	if !hasAccess {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Error checking if principal has access to Condition")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Condition",
			StatusCode: http.StatusForbidden,
		}
	}

	conditionRaw, err := json.Marshal(condition)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: conditionRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        condition.Id,
		Type:      to.Ptr("Condition"),
		Reference: to.Ptr("Condition/" + *condition.Id),
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
