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
	queryParams := request.URL.Query()
	urlParts := strings.Split(strings.TrimPrefix(request.URL.Path, basePath+"/"), "/")
	switch urlParts[0] {
	case "CarePlan":
		// fetch CarePlan + CareTeam, validate requester is participant of CareTeam
		// if there are query params present, we need to handle the request a bit differently
		if len(queryParams) != 0 {
			if queryParams.Get("_id") == "" {
				return coolfhir.NewErrorWithCode("Query parameter _id is not present, can't find CarePlan", http.StatusBadRequest)
			}

			var bundle fhir.Bundle
			err := s.fhirClient.Read("CarePlan", &bundle, fhirclient.QueryParam("_id", queryParams.Get("_id")), fhirclient.QueryParam("_include", "CarePlan:care-team"))
			if err != nil {
				return err
			}
			var careTeams []fhir.CareTeam
			err = coolfhir.ResourcesInBundle(&bundle, coolfhir.EntryIsOfType("CareTeam"), &careTeams)
			if err != nil {
				return err
			}
			if len(careTeams) == 0 {
				return coolfhir.NewErrorWithCode("CareTeam not found in bundle", http.StatusNotFound)
			}

			err = validatePrincipalInCareTeams(request.Context(), careTeams)
			if err != nil {
				return err
			}

			// Valid requester, now perform the bundle request again with all the params
			params := []fhirclient.Option{}
			for k, v := range queryParams {
				params = append(params, fhirclient.QueryParam(k, v[0]))
			}
			err = s.fhirClient.Read("CarePlan", &bundle, params...)
			if err != nil {
				return err
			}

			b, err := json.Marshal(bundle)
			_, err = httpResponse.Write(b)
			if err != nil {
				return err
			}
			return nil
		}
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
