//go:generate go run codegen/main.go

package coolfhir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"

	"net/http"
	"time"
)

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

// ValidateReference validates that a reference is either a logical reference, a reference to a resource, or both.
func ValidateReference(reference fhir.Reference) bool {
	if reference.Reference != nil {
		return true
	}
	return reference.Identifier != nil && reference.Identifier.System != nil && reference.Identifier.Value != nil
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

// IsIdentifierTaskOwnerAndRequester checks the owner and requester of a task against the supplied principal
// returns 2 booleans, isOwner, and isRequester
func IsIdentifierTaskOwnerAndRequester(task *fhir.Task, principalOrganizationIdentifier []fhir.Identifier) (bool, bool) {
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

func IsScpSubTask(task *fhir.Task) bool {
	if task == nil {
		return false
	}
	if len(task.PartOf) == 0 {
		return false
	}
	if task.Meta == nil {
		return false
	}
	return slices.Contains(task.Meta.Profile, SCPTaskProfile)
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
func FindMatchingParticipantInCareTeam(careTeam *fhir.CareTeam, principalOrganizationIdentifiers []fhir.Identifier) *fhir.CareTeamParticipant {
	for _, participant := range careTeam.Participant {
		for _, identifier := range principalOrganizationIdentifiers {
			if IdentifierEquals(participant.Member.Identifier, &identifier) {
				return &participant
			}
		}
	}
	return nil
}

func IsLocalRelativeReference(reference *fhir.Reference) bool {
	return reference != nil && reference.Reference != nil && strings.HasPrefix(*reference.Reference, "#")
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

// ReferenceValueEquals checks if two references are equal based on their reference and type.
func ReferenceValueEquals(ref, other fhir.Reference) bool {
	return ref.Reference != nil && other.Reference != nil && *ref.Reference == *other.Reference &&
		ref.Type != nil && other.Type != nil && *ref.Type == *other.Type
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

// HasIdentifier returns whether a slice of fhir.Identifier contains a specific identifier.
func HasIdentifier(needle fhir.Identifier, haystack ...fhir.Identifier) bool {
	for _, id := range haystack {
		if IdentifierEquals(&needle, &id) {
			return true
		}
	}
	return false
}

func ToString(resource interface{}) string {
	switch r := resource.(type) {
	case *fhir.Identifier:
		return IdentifierToToken(*r)
	case fhir.Identifier:
		return IdentifierToToken(r)
	}
	return fmt.Sprintf("%T(%v)", resource, resource)
}

func IdentifierToToken(r fhir.Identifier) string {
	return fmt.Sprintf("%s|%s", to.EmptyString(r.System), to.EmptyString(r.Value))
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

func IsScpTask(task *fhir.Task) bool {
	if task.Meta == nil {
		return false
	}

	for _, profile := range task.Meta.Profile {
		if profile == SCPTaskProfile {
			return true
		}
	}

	return false
}

// FilterIdentifier returns all identifiers matching the given system from the provided array of identifiers
func FilterIdentifier(identifiers *[]fhir.Identifier, system string) []fhir.Identifier {
	if identifiers == nil {
		return []fhir.Identifier{}
	}
	var result []fhir.Identifier
	for _, identifier := range *identifiers {
		if identifier.System != nil && *identifier.System == system {
			result = append(result, identifier)
		}
	}
	if len(result) == 0 {
		return []fhir.Identifier{}
	}
	return result
}

// FilterFirstIdentifier returns the first identifier matching the given system from the provided array of identifiers
func FilterFirstIdentifier(identifiers *[]fhir.Identifier, system string) *fhir.Identifier {
	if identifiers == nil {
		return nil
	}
	for _, identifier := range *identifiers {
		if identifier.System != nil && *identifier.System == system {
			return &identifier
		}
	}
	return nil
}

// ParseLocalReference parses a local reference and returns the resource type and the resource ID.
// If the reference is not in this format, an error is returned.
func ParseLocalReference(reference string) (string, string, error) {
	if strings.Count(reference, "/") != 1 {
		return "", "", errors.New("local reference must contain exactly one '/'")
	}
	parts := strings.Split(reference, "/")
	if parts[0] == "" {
		return "", "", errors.New("local reference must contain a resource type")
	}
	if parts[1] == "" {
		return "", "", errors.New("local reference must contain a resource ID")
	}
	return parts[0], parts[1], nil
}

// ParseExternalLiteralReference parses an external literal reference and returns the FHIR base URL and the resource reference.
// For example:
// - http://example.com/fhir/Patient/123 yields http://example.com/fhir and Patient/123
// - http://example.com/Patient/123 yields http://example.com and Patient/123
// - http://example.com/subdir/fhir/Patient/123 yields http://example.com/subdir/fhir and Patient/123
// The reference must contain a resource type and an ID, and cannot contain query parameters.
func ParseExternalLiteralReference(literal string, resourceType string) (*url.URL, string, error) {
	parsedLiteral, err := url.Parse(literal)
	if err != nil {
		return nil, "", err
	}
	if len(parsedLiteral.Query()) > 0 {
		return nil, "", errors.New("query parameters for external literal reference are not supported")
	}
	pathParts := strings.Split(parsedLiteral.Path, "/")
	if len(pathParts) < 2 {
		return nil, "", errors.New("external literal reference must contain at least a resource type and an ID")
	}
	if pathParts[len(pathParts)-2] != resourceType {
		return nil, "", errors.New("external literal reference does not contain the expected resource type")
	}
	ref := path.Join(pathParts[len(pathParts)-2:]...)
	parsedLiteral.Path = path.Join(pathParts[:len(pathParts)-2]...)
	return parsedLiteral, ref, nil
}

func SendResponse(httpResponse http.ResponseWriter, httpStatus int, resource interface{}, additionalHeaders ...map[string]string) {
	data, err := json.MarshalIndent(resource, "", "  ")
	if err != nil {
		slog.Error("Failed to marshal response", slog.String("error", err.Error()))
		httpStatus = http.StatusInternalServerError
		data = []byte(`{"resourceType":"OperationOutcome","issue":[{"severity":"error","code":"processing","diagnostics":"Failed to marshal response"}]}`)
	}
	httpResponse.Header().Set("Content-Type", FHIRContentType)
	httpResponse.Header().Set("Content-Length", strconv.Itoa(len(data)))
	for _, hdrs := range additionalHeaders {
		for key, value := range hdrs {
			httpResponse.Header().Add(key, value)
		}
	}
	httpResponse.WriteHeader(httpStatus)
	_, err = httpResponse.Write(data)
	if err != nil {
		slog.Error("Failed to write response", slog.String("error", err.Error()), slog.String("data", string(data)))
	}
}

func ConceptContainsCoding(code fhir.Coding, concepts ...fhir.CodeableConcept) bool {
	for _, codeableConcept := range concepts {
		if ContainsCoding(code, codeableConcept.Coding...) {
			return true
		}
	}
	return false
}

func ContainsCoding(code fhir.Coding, codings ...fhir.Coding) bool {
	for _, currCoding := range codings {
		if (code.Code != nil && currCoding.Code != nil && *code.Code == *currCoding.Code) &&
			(code.System != nil && currCoding.System != nil && *code.System == *currCoding.System) {
			return true
		}
	}
	return false
}

func GetTaskByIdentifier(ctx context.Context, fhirClient fhirclient.Client, identifier fhir.Identifier) (*fhir.Task, error) {
	var existingTaskBundle fhir.Bundle
	headers := http.Header{}
	headers.Add("Cache-Control", "no-cache")
	err := fhirClient.SearchWithContext(ctx, "Task", url.Values{
		"identifier": []string{*identifier.System + "|" + *identifier.Value},
	}, &existingTaskBundle, fhirclient.RequestHeaders(headers))

	if err != nil {
		return nil, fmt.Errorf("failed to search for Task: %v", err)
	}

	if len(existingTaskBundle.Entry) == 1 {
		var existingTask fhir.Task
		if err := ResourceInBundle(&existingTaskBundle, EntryIsOfType("Task"), &existingTask); err != nil {
			return nil, fmt.Errorf("unable to get existing CPS Task resource from search bundle: %v", err)
		}
		slog.DebugContext(ctx, "Found existing CPS Task resource for demo task")
		return &existingTask, nil
	} else if len(existingTaskBundle.Entry) > 1 {
		return nil, fmt.Errorf("found multiple existing CPS Tasks for identifier: %s", *identifier.System+"|"+*identifier.Value)
	}

	return nil, nil
}

// TokenToIdentifier parses a token string into an fhir.Identifier, as specified by https://www.hl7.org/fhir/search.html#token.
// E.g.: system|code, system|, |code
func TokenToIdentifier(s string) (*fhir.Identifier, error) {
	parts := strings.Split(s, "|")
	if len(parts) != 2 {
		return nil, errors.New("identifier search token must contain exactly one '|'")
	}
	if parts[0] == "" && parts[1] == "" {
		return nil, errors.New("identifier search token must contain a system, or a code, or both")
	}
	result := &fhir.Identifier{}
	if parts[0] != "" {
		result.System = &parts[0]
	}
	if parts[1] != "" {
		result.Value = &parts[1]
	}
	return result, nil
}

func OrganizationIdentifiers(organizations []fhir.Organization) []fhir.Identifier {
	var result []fhir.Identifier
	for _, organization := range organizations {
		result = append(result, organization.Identifier...)
	}
	return result
}
