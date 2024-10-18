//go:generate go run codegen/main.go

package coolfhir

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
	"net/http"
	"time"
)

type Task fhir.Task

func (t Task) ToFHIR() (*fhir.Task, error) {
	taskJSON, _ := json.Marshal(t)
	var result fhir.Task
	if err := json.Unmarshal(taskJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshal Task: %w", err)
	}
	return &result, nil
}

var ErrEntryNotFound = errors.New("entry not found in FHIR Bundle")

func LogicalReference(refType, system, identifier string) *fhir.Reference {
	return &fhir.Reference{
		Type: to.Ptr(refType),
		Identifier: &fhir.Identifier{
			System: &system,
			Value:  &identifier,
		},
	}
}

func FirstIdentifier(identifiers []fhir.Identifier, predicate func(fhir.Identifier) bool) *fhir.Identifier {
	for _, identifier := range identifiers {
		if predicate(identifier) {
			return &identifier
		}
	}
	return nil
}

func FilterNamingSystem(system string) func(fhir.Identifier) bool {
	return func(ident fhir.Identifier) bool {
		return ident.System != nil && *ident.System == system
	}
}

func ValidateLogicalReference(reference *fhir.Reference, expectedType string, expectedSystem string) error {
	if reference == nil {
		return errors.New("reference cannot be nil")
	}
	if reference.Type == nil || *reference.Type != expectedType {
		return fmt.Errorf("reference.Type must be %s", expectedType)
	}
	if reference.Identifier == nil || reference.Identifier.System == nil || reference.Identifier.Value == nil {
		return errors.New("reference must contain a logical identifier with a System and Value")
	}
	if *reference.Identifier.System != expectedSystem {
		return fmt.Errorf("reference.Identifier.System must be %s", expectedSystem)
	}
	return nil
}

// ValidateCareTeamParticipantPeriod validates that a CareTeamParticipant has a start date, and that the start date is in the past
// end date is not required, but if present it will validate that it is in the future
func ValidateCareTeamParticipantPeriod(participant fhir.CareTeamParticipant, now time.Time) (bool, error) {
	// Member must have start date, this date must be in the past, and if there is an end date then it must be in the future
	if participant.Period == nil {
		return false, errors.New("CareTeamParticipant has nil period")
	}
	if participant.Period.Start == nil {
		return false, errors.New("CareTeamParticipant has nil start date")
	}

	startTime, err := parseTimestamp(*participant.Period.Start)
	if err != nil {
		return false, err
	}
	if !now.After(startTime) {
		return false, nil
	}
	if participant.Period.End != nil {
		endTime, err := parseTimestamp(*participant.Period.End)
		if err != nil {
			return false, err
		}
		if !now.Before(endTime) {
			return false, nil
		}
	}
	return true, nil
}

// ValidateTaskRequiredFields Validates that all required fields are set for a Task (i.e. a cardinality of 1..*) as per: https://santeonnl.github.io/shared-care-planning/StructureDefinition-SCPTask.html
// and that the value is valid
func ValidateTaskRequiredFields(task fhir.Task) error {
	if task.Intent != "order" {
		return errors.New("task.Intent must be 'order'")
	}
	if task.For != nil {
		err := validateIdentifier("task.For", task.For.Identifier)
		if err != nil {
			return err
		}
	}
	if task.Requester != nil {
		err := validateIdentifier("task.Requester", task.Requester.Identifier)
		if err != nil {
			return err
		}
	}
	if task.Owner != nil {
		err := validateIdentifier("task.Owner", task.Owner.Identifier)
		if err != nil {
			return err
		}
	}
	return nil
}

// ValidateTaskOwnerAndRequester checks the owner and requester of a task against the supplied principal
// returns 2 booleans, isOwner, and isRequester
func ValidateTaskOwnerAndRequester(task *fhir.Task, principalOrganizationIdentifier []fhir.Identifier) (bool, bool) {
	isOwner := false
	if task.Owner != nil {
		for _, identifier := range principalOrganizationIdentifier {
			if LogicalReferenceEquals(*task.Owner, fhir.Reference{Identifier: &identifier}) {
				isOwner = true
				break
			}
		}
	}
	isRequester := false
	if task.Requester != nil {
		for _, identifier := range principalOrganizationIdentifier {
			if LogicalReferenceEquals(*task.Requester, fhir.Reference{Identifier: &identifier}) {
				isRequester = true
				break
			}
		}
	}
	return isOwner, isRequester
}

func validateIdentifier(identifierField string, identifier *fhir.Identifier) error {
	if identifier == nil {
		return nil
	}
	if identifier.System == nil || *identifier.System == "" {
		return errors.New(identifierField + ".Identifier.System must be set")
	}
	if identifier.Value == nil || *identifier.Value == "" {
		return errors.New(identifierField + ".Identifier.Value must be set")
	}
	return nil
}

func parseTimestamp(timestampString string) (time.Time, error) {
	// Check both yyyy-mm-dd and extended with full timestamp
	var timeStamp time.Time
	var err error
	if len(timestampString) > 10 {
		timeStamp, err = time.Parse(time.RFC3339, timestampString)
		if err != nil {
			return time.Time{}, err
		}
	} else if len(timestampString) == 10 {
		timeStamp, err = time.Parse(time.DateOnly, timestampString)
		if err != nil {
			return time.Time{}, err
		}
	} else {
		return time.Time{}, errors.New("unsupported timestamp format")
	}
	return timeStamp, nil
}

// FindMatchingParticipantInCareTeam loops through each Participant of each CareTeam present in the CarePlan, trying to match it to the Principal Organization Identifiers provided
// if a valid Participant is found, return it. This can be used for further validation e.g. for the Period
func FindMatchingParticipantInCareTeam(careTeams []fhir.CareTeam, principalOrganizationIdentifiers []fhir.Identifier) *fhir.CareTeamParticipant {
	for _, careTeam := range careTeams {
		for _, participant := range careTeam.Participant {
			for _, identifier := range principalOrganizationIdentifiers {
				if IdentifierEquals(participant.Member.Identifier, &identifier) {
					return &participant
				}
			}
		}
	}
	return nil
}

func IsLogicalReference(reference *fhir.Reference) bool {
	return reference != nil && reference.Type != nil && IsLogicalIdentifier(reference.Identifier)
}

func IsLogicalIdentifier(identifier *fhir.Identifier) bool {
	return identifier != nil && identifier.System != nil && identifier.Value != nil
}

// LogicalReferenceEquals checks if two references are contain the same logical identifier, given their system and value.
// It does not compare identifier type.
func LogicalReferenceEquals(ref, other fhir.Reference) bool {
	return ref.Identifier != nil && other.Identifier != nil &&
		ref.Identifier.System != nil && other.Identifier.System != nil && *ref.Identifier.System == *other.Identifier.System &&
		ref.Identifier.Value != nil && other.Identifier.Value != nil && *ref.Identifier.Value == *other.Identifier.Value
}

// IdentifierEquals compares two logical identifiers based on their system and value.
// If any of the identifiers is nil or any of the system or value fields is nil, it returns false.
// If the system and value fields of both identifiers are equal, it returns true.
func IdentifierEquals(one *fhir.Identifier, other *fhir.Identifier) bool {
	if one == nil || other == nil {
		return false
	}
	if one.System == nil || other.System == nil {
		return false
	}
	if one.Value == nil || other.Value == nil {
		return false
	}
	return *one.System == *other.System && *one.Value == *other.Value
}

func ToString(resource interface{}) string {
	switch r := resource.(type) {
	case *fhir.Identifier:
		return fmt.Sprintf("%s|%s", *r.System, *r.Value)
	case fhir.Identifier:
		return fmt.Sprintf("%s|%s", *r.System, *r.Value)
	}
	return fmt.Sprintf("%T(%v)", resource, resource)
}

func HttpMethodToVerb(method string) fhir.HTTPVerb {
	switch method {
	case http.MethodPut:
		return fhir.HTTPVerbPUT
	case http.MethodPost:
		return fhir.HTTPVerbPOST
	case http.MethodDelete:
		return fhir.HTTPVerbDELETE
	case http.MethodPatch:
		return fhir.HTTPVerbPATCH
	case http.MethodGet:
		return fhir.HTTPVerbGET
	case http.MethodHead:
		return fhir.HTTPVerbHEAD
	default:
		return 0
	}
}
