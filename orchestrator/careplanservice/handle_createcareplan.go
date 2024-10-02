package careplanservice

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleCreateCarePlan(httpRequest *http.Request, tx *coolfhir.TransactionBuilder) (FHIRHandlerResult, error) {
	log.Info().Msg("Creating CarePlan")
	// TODO: Authorize request here
	// TODO: Check only allowed fields are set, or only the allowed values (INT-204)?
	var carePlan fhir.CarePlan
	if err := s.readRequest(httpRequest, &carePlan); err != nil {
		return nil, fmt.Errorf("invalid %T: %w", carePlan, err)
	}
	// Reset CarePlan.Id to nil to ensure it is generated by the server
	carePlan.Id = nil

	careTeamURL := "urn:uuid:" + uuid.NewString()
	careTeam := fhir.CareTeam{}
	carePlan.CareTeam = append(carePlan.CareTeam, fhir.Reference{
		Reference: to.Ptr(careTeamURL),
		Type:      to.Ptr(coolfhir.ResourceType(careTeam)),
	})

	tx.Create(carePlan).Create(careTeam, coolfhir.WithFullUrl(careTeamURL))

	return func(txResult *fhir.Bundle) (*fhir.BundleEntry, []any, error) {
		result, err := coolfhir.FetchBundleEntry(s.fhirClient, txResult, func(entry fhir.BundleEntry) bool {
			return entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "CarePlan/")
		}, nil)
		return result, nil, err
	}, nil
}
