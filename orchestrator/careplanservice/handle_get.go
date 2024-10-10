package careplanservice

import (
	"context"
	"encoding/json"
	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/auth"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"strings"
)

func (s *Service) handleGet(httpResponse http.ResponseWriter, request *http.Request) error {
	urlParts := strings.Split(strings.TrimPrefix(request.URL.Path, basePath+"/"), "/")
	switch urlParts[0] {
	case "CarePlan":
		// fetch CarePlan + CareTeam, validate requester is participant of CareTeam
		if len(urlParts) < 2 {
			return &coolfhir.ErrorWithCode{
				Message:    "URL missing CarePlan ID",
				StatusCode: http.StatusForbidden,
			}
		}
		carePlan, careTeams, err := s.getCarePlanAndCareTeams("CarePlan/" + urlParts[1])
		if err != nil {
			return err
		}

		err = validatePrincipalInCareTeams(request.Context(), careTeams)
		if err != nil {
			return err
		}

		b, err := json.Marshal(carePlan)
		_, err = httpResponse.Write(b)
		if err != nil {
			return err
		}
		// TODO: Do we support additional info in the URL i.e. extra params
	case "CareTeam":
		// fetch CareTeam, validate requester is participant
		if len(urlParts) < 2 {
			return &coolfhir.ErrorWithCode{
				Message:    "URL missing CareTeam ID",
				StatusCode: http.StatusForbidden,
			}
		}
		var careTeam fhir.CareTeam
		err := s.fhirClient.Read("CareTeam/"+urlParts[1], &careTeam)
		if err != nil {
			return err
		}
		err = validatePrincipalInCareTeams(request.Context(), []fhir.CareTeam{careTeam})
		if err != nil {
			return err
		}

		b, err := json.Marshal(careTeam)
		_, err = httpResponse.Write(b)
		if err != nil {
			return err
		}
		// TODO: Do we support additional info in the URL i.e. extra params

	case "Task":
		// fetch Task + CareTeam, validate requester is participant of CareTeam
		if len(urlParts) < 2 {
			return &coolfhir.ErrorWithCode{
				Message:    "URL missing Task ID",
				StatusCode: http.StatusForbidden,
			}
		}
		var task fhir.Task
		// var basedOn []fhir.Reference
		err := s.fhirClient.Read("Task/"+urlParts[1], &task)
		if err != nil {
			return err
		}
		// This shouldn't be possible, but still worth checking
		if len(task.BasedOn) != 1 {
			return &coolfhir.ErrorWithCode{
				Message:    "Task has invalid number of BasedOn values",
				StatusCode: http.StatusInternalServerError,
			}
		}
		if task.BasedOn[0].Reference == nil {
			return &coolfhir.ErrorWithCode{
				Message:    "Task has invalid BasedOn Reference",
				StatusCode: http.StatusInternalServerError,
			}
		}

		_, careTeams, err := s.getCarePlanAndCareTeams(*task.BasedOn[0].Reference)
		if err != nil {
			return err
		}

		err = validatePrincipalInCareTeams(request.Context(), careTeams)
		if err != nil {
			return err
		}

		b, err := json.Marshal(task)
		_, err = httpResponse.Write(b)
		if err != nil {
			return err
		}
		// TODO: Do we support additional info in the URL i.e. extra params
	default:
		log.Warn().Msgf("Unmanaged FHIR operation at CarePlanService: %s %s", request.Method, request.URL.String())
		s.proxy.ServeHTTP(httpResponse, request)
	}
	return nil
}

func (s *Service) getCarePlanAndCareTeams(carePlanReference string) (fhir.CarePlan, []fhir.CareTeam, error) {
	var carePlan fhir.CarePlan
	var careTeams []fhir.CareTeam
	err := s.fhirClient.Read(carePlanReference, &carePlan, fhirclient.ResolveRef("careTeam", &careTeams))
	if err != nil {
		return fhir.CarePlan{}, nil, err
	}
	if len(careTeams) == 0 {
		return fhir.CarePlan{}, nil, &coolfhir.ErrorWithCode{
			Message:    "CareTeam not found in bundle",
			StatusCode: http.StatusNotFound,
		}
	}

	return carePlan, careTeams, nil
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
			Message:    "Participant is not part of CarePlan",
			StatusCode: http.StatusUnauthorized,
		}
	}
	return nil
}
