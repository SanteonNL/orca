package careplanservice

type Policy interface {
	HasAccess() (bool, error)
}

var _ Policy = &AnyMatchPolicy{}

// AnyMatchPolicy is a policy that allows access if any of the policies in the list allow access.
type AnyMatchPolicy []Policy

func (e AnyMatchPolicy) HasAccess() (bool, error) {
	for _, policy := range e {
		hasAccess, err := policy.HasAccess()
		if err != nil {
			return false, err
		}
		if hasAccess {
			return true, nil
		}
	}
	return false, nil
}

var _ Policy = &CreatorHasAccess{}

type EveryoneHasAccessPolicy struct {
}

func (e EveryoneHasAccessPolicy) HasAccess() (bool, error) {
	return true, nil
}

var _ Policy = &EveryoneHasAccessPolicy{}

type CreatorHasAccess struct {
}

func (o CreatorHasAccess) HasAccess() (bool, error) {
	// TODO: Find a more suitable way to handle this auth.
	// The AuditEvent implementation has proven unsuitable and we are using the AuditEvent for unintended purposes.
	// For now, we can return true, as this will follow the same logic as was present before implementing the AuditEvent.

	return true, nil

	//var auditBundle fhir.Bundle
	//err := s.fhirClient.SearchWithContext(ctx, "AuditEvent", url.Values{
	//	"entity": []string{resourceType + "/" + resourceID},
	//	"action": []string{fhir.AuditEventActionC.String()},
	//}, &auditBundle)
	//if err != nil {
	//	return false, fmt.Errorf("failed to find creation AuditEvent: %w", err)
	//}
	//
	//// Check if there's a creation audit event
	//if len(auditBundle.Entry) == 0 {
	//	return false, coolfhir.NewErrorWithCode(fmt.Sprintf("No creation audit event found for %s", resourceType), http.StatusForbidden)
	//}
	//
	//// Get the creator from the audit event
	//var creationAuditEvent fhir.AuditEvent
	//err = json.Unmarshal(auditBundle.Entry[0].Resource, &creationAuditEvent)
	//if err != nil {
	//	return false, fmt.Errorf("failed to unmarshal AuditEvent: %w", err)
	//}
	//
	//// Check if the current user is the creator
	//if !audit.IsCreator(creationAuditEvent, &principal) {
	//	return false, coolfhir.NewErrorWithCode(fmt.Sprintf("Only the creator can update this %s", resourceType), http.StatusForbidden)
	//}
	//
	//return true, nil
}
