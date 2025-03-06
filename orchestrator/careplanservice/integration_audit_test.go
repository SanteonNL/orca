package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_CRUD_AuditEvents(t *testing.T) {
	// Setup test clients and service
	cpc1NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	cpc2NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	carePlanContributor1, carePlanContributor2, _, _ := setupIntegrationTest(t, cpc1NotificationEndpoint, cpc2NotificationEndpoint)

	// Track all expected audit events
	var expectedAuditEvents []ExpectedAuditEvent

	// Helper to add expected audit events
	addExpectedAudit := func(resourceRef string, action fhir.AuditEventAction) {
		expectedAuditEvents = append(expectedAuditEvents, ExpectedAuditEvent{
			ResourceRef: resourceRef,
			Action:      action,
		})
	}

	// Create Patient
	patient := fhir.Patient{
		Identifier: []fhir.Identifier{
			{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Name: []fhir.HumanName{
			{
				Given:  []string{"Test"},
				Family: to.Ptr("Patient"),
			},
		},
	}
	err := carePlanContributor1.Create(patient, &patient)
	require.NoError(t, err)
	addExpectedAudit("Patient/"+*patient.Id, fhir.AuditEventActionC)

	// Create Questionnaire
	questionnaire := fhir.Questionnaire{
		Status: fhir.PublicationStatusDraft,
		Title:  to.Ptr("Test Questionnaire"),
		Item: []fhir.QuestionnaireItem{
			{
				LinkId: "1",
				Text:   to.Ptr("Question 1"),
				Type:   fhir.QuestionnaireItemTypeString,
			},
		},
	}
	err = carePlanContributor1.Create(questionnaire, &questionnaire)
	require.NoError(t, err)
	addExpectedAudit("Questionnaire/"+*questionnaire.Id, fhir.AuditEventActionC)

	// Create QuestionnaireResponse
	questionnaireResponse := fhir.QuestionnaireResponse{
		Status:        fhir.QuestionnaireResponseStatusInProgress,
		Questionnaire: to.Ptr("Questionnaire/" + *questionnaire.Id),
		Subject: &fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
	}
	err = carePlanContributor1.Create(questionnaireResponse, &questionnaireResponse)
	require.NoError(t, err)
	addExpectedAudit("QuestionnaireResponse/"+*questionnaireResponse.Id, fhir.AuditEventActionC)

	// Create ServiceRequest
	serviceRequest := fhir.ServiceRequest{
		Status: fhir.RequestStatusActive,
		Intent: fhir.RequestIntentOrder,
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Code: &fhir.CodeableConcept{
			Text: to.Ptr("Test Service"),
		},
	}
	err = carePlanContributor1.Create(serviceRequest, &serviceRequest)
	require.NoError(t, err)
	addExpectedAudit("ServiceRequest/"+*serviceRequest.Id, fhir.AuditEventActionC)

	// Update Patient
	patient.Name[0].Given = []string{"Updated"}
	err = carePlanContributor1.Update("Patient/"+*patient.Id, patient, &patient)
	require.NoError(t, err)
	addExpectedAudit("Patient/"+*patient.Id, fhir.AuditEventActionU)

	// Update Questionnaire
	questionnaire.Title = to.Ptr("Updated Questionnaire")
	err = carePlanContributor1.Update("Questionnaire/"+*questionnaire.Id, questionnaire, &questionnaire)
	require.NoError(t, err)
	addExpectedAudit("Questionnaire/"+*questionnaire.Id, fhir.AuditEventActionU)

	// Update QuestionnaireResponse
	questionnaireResponse.Status = fhir.QuestionnaireResponseStatusCompleted
	err = carePlanContributor1.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponse, &questionnaireResponse)
	require.NoError(t, err)
	addExpectedAudit("QuestionnaireResponse/"+*questionnaireResponse.Id, fhir.AuditEventActionU)

	// Update ServiceRequest
	serviceRequest.Status = fhir.RequestStatusCompleted
	err = carePlanContributor1.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequest, &serviceRequest)
	require.NoError(t, err)
	addExpectedAudit("ServiceRequest/"+*serviceRequest.Id, fhir.AuditEventActionU)

	// Negative tests - different user trying to update resources
	t.Run("Update Patient with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("Patient/"+*patient.Id, patient, &patient)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this Patient")
	})

	t.Run("Update Questionnaire with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("Questionnaire/"+*questionnaire.Id, questionnaire, &questionnaire)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this Questionnaire")
	})

	t.Run("Update QuestionnaireResponse with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponse, &questionnaireResponse)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this QuestionnaireResponse")
	})

	t.Run("Update ServiceRequest with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequest, &serviceRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Only the creator can update this ServiceRequest")
	})

	// Update non-existing resources (creates new ones)
	t.Run("Update non-existing Patient - creates new resource", func(t *testing.T) {
		nonExistingPatient := fhir.Patient{
			Id: to.Ptr("non-existing-patient"),
			Identifier: []fhir.Identifier{
				{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333338"),
				},
			},
			Name: []fhir.HumanName{
				{
					Given:  []string{"New"},
					Family: to.Ptr("Patient"),
				},
			},
		}
		err = carePlanContributor1.Update("Patient/"+*nonExistingPatient.Id, nonExistingPatient, &nonExistingPatient)
		require.NoError(t, err)
		addExpectedAudit("Patient/"+*nonExistingPatient.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing Questionnaire - creates new resource", func(t *testing.T) {
		nonExistingQuestionnaire := fhir.Questionnaire{
			Id:     to.Ptr("non-existing-questionnaire"),
			Status: fhir.PublicationStatusDraft,
			Title:  to.Ptr("New Test Questionnaire"),
			Item: []fhir.QuestionnaireItem{
				{
					LinkId: "1",
					Text:   to.Ptr("New Question 1"),
					Type:   fhir.QuestionnaireItemTypeString,
				},
			},
		}
		err = carePlanContributor1.Update("Questionnaire/"+*nonExistingQuestionnaire.Id, nonExistingQuestionnaire, &nonExistingQuestionnaire)
		require.NoError(t, err)
		addExpectedAudit("Questionnaire/"+*nonExistingQuestionnaire.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing QuestionnaireResponse - creates new resource", func(t *testing.T) {
		nonExistingQuestionnaireResponse := fhir.QuestionnaireResponse{
			Id:            to.Ptr("non-existing-questionnaire-response"),
			Status:        fhir.QuestionnaireResponseStatusInProgress,
			Questionnaire: to.Ptr("Questionnaire/" + *questionnaire.Id),
			Subject: &fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
		}
		err = carePlanContributor1.Update("QuestionnaireResponse/"+*nonExistingQuestionnaireResponse.Id, nonExistingQuestionnaireResponse, &nonExistingQuestionnaireResponse)
		require.NoError(t, err)
		addExpectedAudit("QuestionnaireResponse/"+*nonExistingQuestionnaireResponse.Id, fhir.AuditEventActionC)
	})

	t.Run("Update non-existing ServiceRequest - creates new resource", func(t *testing.T) {
		nonExistingServiceRequest := fhir.ServiceRequest{
			Id:     to.Ptr("non-existing-service-request"),
			Intent: fhir.RequestIntentOrder,
			Status: fhir.RequestStatusDraft,
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Code: &fhir.CodeableConcept{
				Text: to.Ptr("New Service Request"),
			},
		}
		err = carePlanContributor1.Update("ServiceRequest/"+*nonExistingServiceRequest.Id, nonExistingServiceRequest, &nonExistingServiceRequest)
		require.NoError(t, err)
		addExpectedAudit("ServiceRequest/"+*nonExistingServiceRequest.Id, fhir.AuditEventActionC)
	})

	// Verify all audit events at the end
	err = verifyAuditEvents(t, carePlanContributor1, expectedAuditEvents)
	require.NoError(t, err)
}

// Define a new type to hold expected audit events without timestamp requirements
type ExpectedAuditEvent struct {
	ResourceRef string
	Action      fhir.AuditEventAction
	QueryParams map[string][]string
}

// Refactored verifyAuditEvents to handle a list of expected audit events without timestamp requirements
func verifyAuditEvents(t *testing.T, fhirClient fhirclient.Client, expectedEvents []ExpectedAuditEvent) error {
	t.Helper()

	// Create a context with timeout to avoid hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Log the search attempt for debugging
	t.Logf("Searching for AuditEvents")

	var bundle fhir.Bundle
	err := fhirClient.SearchWithContext(ctx, "AuditEvent", url.Values{}, &bundle)

	if err != nil {
		return fmt.Errorf("failed to search AuditEvents: %w", err)
	}

	// Log success for debugging
	t.Logf("Successfully retrieved %d AuditEvents", len(bundle.Entry))

	// Track which expected events have been found
	foundEvents := make(map[string]bool)

	// Process each audit event in the bundle
	for _, entry := range bundle.Entry {
		var auditEvent fhir.AuditEvent
		if err := json.Unmarshal(entry.Resource, &auditEvent); err != nil {
			return fmt.Errorf("failed to unmarshal AuditEvent: %w", err)
		}

		// Skip if no entities or action
		if len(auditEvent.Entity) == 0 || auditEvent.Action == nil {
			continue
		}

		// Check each entity in the audit event
		for _, entity := range auditEvent.Entity {
			if entity.What == nil || entity.What.Reference == nil {
				continue
			}

			resourceRef := *entity.What.Reference
			actionKey := fmt.Sprintf("%s:%s", resourceRef, *auditEvent.Action)

			// Check if this matches any expected event
			for _, expectedEvent := range expectedEvents {
				expectedKey := fmt.Sprintf("%s:%s", expectedEvent.ResourceRef, expectedEvent.Action)

				if actionKey == expectedKey {
					// Check query parameters if needed
					if expectedEvent.QueryParams != nil && *auditEvent.Action == fhir.AuditEventActionE {
						paramsMatch := verifyQueryParams(auditEvent, expectedEvent.QueryParams)
						if !paramsMatch {
							continue
						}
					}

					// Mark this expected event as found
					foundEvents[expectedKey] = true
					break
				}
			}
		}
	}

	// Check if all expected events were found
	for _, event := range expectedEvents {
		key := fmt.Sprintf("%s:%s", event.ResourceRef, event.Action)
		if !foundEvents[key] {
			return fmt.Errorf("expected to find audit event with action %s for resource %s",
				event.Action, event.ResourceRef)
		}
	}

	return nil
}

// Helper function to verify query parameters in an audit event
func verifyQueryParams(auditEvent fhir.AuditEvent, queryParams map[string][]string) bool {
	// Find query entity
	for _, e := range auditEvent.Entity {
		if e.Type != nil && e.Type.Code != nil && *e.Type.Code == "2" { // "2" is the code for query parameters
			// Verify all expected params exist in details
			for param, values := range queryParams {
				paramFound := false
				for _, detail := range e.Detail {
					if detail.Type == param && *detail.ValueString == strings.Join(values, ",") {
						paramFound = true
						break
					}
				}
				if !paramFound {
					return false
				}
			}
			return true
		}
	}
	return false
}
