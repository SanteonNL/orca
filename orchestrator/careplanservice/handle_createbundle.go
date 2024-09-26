package careplanservice

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/rs/zerolog/log"
	"github.com/samply/golang-fhir-models/fhir-models/fhir"
)

func (s *Service) handleBundle(httpResponse http.ResponseWriter, httpRequest *http.Request) error {
	log.Info().Msg("in de handleCreateBundle")
	// TODO: Authorize request here
	// TODO: Make this a valid implementation, currently only handling the subtask with task.output scenario and not executing all in one transaction
	// TODO: Unpack Bundle and execute each entry separately, calling the appropriate handlers. The handlers should operate on a `fhir.Transaction`, which is executed by this function. Otherwise, we allow uncontrolled access through bundles, allow state to desync (e.g. because the CareTeam isn't updated, part of a Task acceptance) and inconsistent state if some entries are applied, and others not (hence the Transaction).
	var bundle fhir.Bundle
	if err := s.readRequest(httpRequest, &bundle); err != nil {
		return fmt.Errorf("invalid %T: %w", bundle, err)
	}
	log.Trace().Msg("converted Bundle to fhir.Bundle")

	//Simply execute the bundle for now - extract the Task if it's in the bundle
	var updatedTask fhir.Task
	err := coolfhir.ExecuteTransactionAndRespondWithEntry(s.fhirClient, bundle, func(entry fhir.BundleEntry) bool {
		//TODO: Add  && entry.Request.Method == fhir.HTTPVerbPUT req
		return entry.Response != nil && entry.Response.Location != nil && strings.HasPrefix(*entry.Response.Location, "Task/")
	}, httpResponse, &updatedTask)

	if err != nil {
		return fmt.Errorf("failed to create Bundle: %w", err)
	}
	log.Info().Msg("Executed Bundle")

	if updatedTask.Id != nil {
		log.Trace().Msg("Found updated task")
		// s.handleUpdateTaskById(updatedTask["id"].(string), httpResponse, httpRequest)
		s.handleTaskFillerUpdate(&updatedTask)
	}

	return nil
}
