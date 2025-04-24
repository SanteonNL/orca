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
	"net/url"
)

func ReadPatientAuthzPolicy(fhirClient fhirclient.Client) Policy[fhir.Patient] {
	return RelatedResourceSearchPolicy[fhir.Patient, fhir.CarePlan]{
		fhirClient:            fhirClient,
		relatedResourcePolicy: CareTeamMemberPolicy[fhir.CarePlan]{},
		relatedResourceSearchParams: func(ctx context.Context, resource fhir.Patient) (resourceType string, searchParams *url.Values) {
			return "CarePlan", &url.Values{"subject": []string{"Patient/" + *resource.Id}}
		},
	}
}

// handleReadPatient fetches the requested Patient and validates if the requester has access to the resource (is a participant of one of the CareTeams associated with the patient)
// if the requester is valid, return the Patient, else return an error
// Pass in a pointer to a fhirclient.Headers object to get the headers from the fhir client request
func (s *Service) handleReadPatient(ctx context.Context, request FHIRHandlerRequest, tx *coolfhir.BundleBuilder) (FHIRHandlerResult, error) {
	log.Ctx(ctx).Info().Msgf("Getting Patient with ID: %s", request.ResourceId)
	var patient fhir.Patient
	err := s.fhirClient.ReadWithContext(ctx, "Patient/"+request.ResourceId, &patient, fhirclient.ResponseHeaders(request.FhirHeaders))
	if err != nil {
		return nil, err
	}

	hasAccess, err := ReadPatientAuthzPolicy(s.fhirClient).HasAccess(ctx, patient, *request.Principal)
	if !hasAccess {
		if err != nil {
			log.Ctx(ctx).Error().Err(err).Msg("Error checking if principal has access to Patient")
		}
		return nil, &coolfhir.ErrorWithCode{
			Message:    "Participant does not have access to Patient",
			StatusCode: http.StatusForbidden,
		}
	}

	patientRaw, err := json.Marshal(patient)
	if err != nil {
		return nil, err
	}

	bundleEntry := fhir.BundleEntry{
		Resource: patientRaw,
		Response: &fhir.BundleEntryResponse{
			Status: "200 OK",
		},
	}

	auditEvent := audit.Event(*request.LocalIdentity, fhir.AuditEventActionR, &fhir.Reference{
		Id:        patient.Id,
		Type:      to.Ptr("Patient"),
		Reference: to.Ptr("Patient/" + *patient.Id),
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
