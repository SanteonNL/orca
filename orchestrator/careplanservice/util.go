package careplanservice

import (
	"context"
	"fmt"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"net/http"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

// Writes an OperationOutcome based on the given error as HTTP response.
func (s *Service) writeOperationOutcomeFromError(err error, desc string, httpResponse http.ResponseWriter) {
	log.Info().Msgf("%s failed: %v", desc, err)
	diagnostics := fmt.Sprintf("%s failed: %s", desc, err.Error())

	issue := fhir.OperationOutcomeIssue{
		Severity:    fhir.IssueSeverityError,
		Code:        fhir.IssueTypeProcessing,
		Diagnostics: to.Ptr(diagnostics),
	}

	outcome := fhir.OperationOutcome{
		Issue: []fhir.OperationOutcomeIssue{issue},
	}

	coolfhir.SendResponse(httpResponse, http.StatusBadRequest, outcome)
}

func (s *Service) getCarePlanAndCareTeams(carePlanReference string) (fhir.CarePlan, []fhir.CareTeam, *fhirclient.Headers, error) {
	var carePlan fhir.CarePlan
	var careTeams []fhir.CareTeam
	headers := new(fhirclient.Headers)
	err := s.fhirClient.Read(carePlanReference, &carePlan, fhirclient.ResolveRef("careTeam", &careTeams), fhirclient.ResponseHeaders(headers))
	if err != nil {
		return fhir.CarePlan{}, nil, nil, err
	}
	if len(careTeams) == 0 {
		return fhir.CarePlan{}, nil, nil, &coolfhir.ErrorWithCode{
			Message:    "CareTeam not found in bundle",
			StatusCode: http.StatusNotFound,
		}
	}

	return carePlan, careTeams, headers, nil
}

func validatePrincipalInCareTeams(ctx context.Context, careTeams []fhir.CareTeam) error {
	// Verify requester is in CareTeams
	principal, err := auth.PrincipalFromContext(ctx)
	if err != nil {
		return err
	}
	participant := coolfhir.FindMatchingParticipantInCareTeam(careTeams, principal.Organization.Identifier)
	if participant == nil {
		return &coolfhir.ErrorWithCode{
			Message:    "Participant is not part of CareTeam",
			StatusCode: http.StatusForbidden,
		}
	}
	return nil
}

// matchResourceIDs matches whether the ID in the request URL matches the ID in the resource.
// This is important for PUT requests, where the ID in the URL is the ID of the resource to update.
// They do not need to be set both, but if they are, they should match.
func matchResourceIDs(request *FHIRHandlerRequest, idFromResource *string) error {
	if (idFromResource != nil && request.ResourceId != "") && request.ResourceId != *idFromResource {
		return &coolfhir.ErrorWithCode{
			Message:    "ID in request URL does not match ID in resource",
			StatusCode: http.StatusBadRequest,
		}
	}
	return nil
}
