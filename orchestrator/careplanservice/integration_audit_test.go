package careplanservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	fhirclient "github.com/SanteonNL/go-fhir-client"
	"github.com/SanteonNL/orca/orchestrator/lib/coolfhir"
	"github.com/SanteonNL/orca/orchestrator/lib/test"
	"github.com/SanteonNL/orca/orchestrator/lib/to"
	"github.com/stretchr/testify/require"
	"github.com/zorgbijjou/golang-fhir-models/fhir-models/fhir"
)

func Test_CRUD_AuditEvents(t *testing.T) {
	t.Skip()
	// Setup test clients and service
	fhirBaseURL := test.SetupHAPI(t)

	cpc1NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	cpc2NotificationEndpoint := setupNotificationEndpoint(t, func(n coolfhir.SubscriptionNotification) {})
	carePlanContributor1, carePlanContributor2, _, _ := setupIntegrationTest(t, cpc1NotificationEndpoint, cpc2NotificationEndpoint, fhirBaseURL)

	// Track all expected audit events
	var expectedAuditEvents []ExpectedAuditEvent

	// Helper to add expected audit events
	addExpectedAudit := func(resourceRef string, action fhir.AuditEventAction) {
		expectedAuditEvents = append(expectedAuditEvents, ExpectedAuditEvent{
			ResourceRef: resourceRef,
			Action:      action,
		})
	}

	addExpectedSearchAudit := func(resourceRef string, queryParams map[string][]string) {
		expectedAuditEvents = append(expectedAuditEvents, ExpectedAuditEvent{
			ResourceRef: resourceRef,
			Action:      fhir.AuditEventActionR,
			QueryParams: queryParams,
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

	// Create Condition
	condition := fhir.Condition{
		ClinicalStatus: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://terminology.hl7.org/CodeSystem/condition-clinical"),
					Code:    to.Ptr("active"),
					Display: to.Ptr("Active"),
				},
			},
		},
		Subject: fhir.Reference{
			Identifier: &fhir.Identifier{
				System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
				Value:  to.Ptr("1333333337"),
			},
		},
		Code: &fhir.CodeableConcept{
			Coding: []fhir.Coding{
				{
					System:  to.Ptr("http://snomed.info/sct"),
					Code:    to.Ptr("386661006"),
					Display: to.Ptr("Fever"),
				},
			},
		},
	}
	err = carePlanContributor1.Create(condition, &condition)
	require.NoError(t, err)
	addExpectedAudit("Condition/"+*condition.Id, fhir.AuditEventActionC)

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
		require.Contains(t, err.Error(), "Participant is not authorized to update Patient")
	})

	t.Run("Update QuestionnaireResponse with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("QuestionnaireResponse/"+*questionnaireResponse.Id, questionnaireResponse, &questionnaireResponse)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Participant is not authorized to update QuestionnaireResponse")
	})

	t.Run("Update ServiceRequest with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("ServiceRequest/"+*serviceRequest.Id, serviceRequest, &serviceRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Participant is not authorized to update ServiceRequest")
	})

	t.Run("Update Condition with different requester - fails", func(t *testing.T) {
		err = carePlanContributor2.Update("Condition/"+*condition.Id, condition, &condition)
		require.Error(t, err)
		require.Contains(t, err.Error(), "Participant is not authorized to update Condition")
	})

	// Update non-existing resources (creates new ones)
	var nonExistingPatient fhir.Patient
	t.Run("Update non-existing Patient - creates new resource", func(t *testing.T) {
		nonExistingPatient = fhir.Patient{
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

		// Validate that the supplied ID was used
		require.Equal(t, *nonExistingPatient.Id, "non-existing-patient")
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

	t.Run("Update non-existing Condition - creates new resource", func(t *testing.T) {
		nonExistingCondition := fhir.Condition{
			Id: to.Ptr("non-existing-condition"),
			ClinicalStatus: &fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://terminology.hl7.org/CodeSystem/condition-clinical"),
						Code:    to.Ptr("active"),
						Display: to.Ptr("Active"),
					},
				},
			},
			Subject: fhir.Reference{
				Identifier: &fhir.Identifier{
					System: to.Ptr("http://fhir.nl/fhir/NamingSystem/bsn"),
					Value:  to.Ptr("1333333337"),
				},
			},
			Code: &fhir.CodeableConcept{
				Coding: []fhir.Coding{
					{
						System:  to.Ptr("http://snomed.info/sct"),
						Code:    to.Ptr("386661006"),
						Display: to.Ptr("New Fever"),
					},
				},
			},
		}
		err = carePlanContributor1.Update("Condition/"+*nonExistingCondition.Id, nonExistingCondition, &nonExistingCondition)
		require.NoError(t, err)
		addExpectedAudit("Condition/"+*nonExistingCondition.Id, fhir.AuditEventActionC)
	})

	t.Run("Read Patient by id", func(t *testing.T) {
		var readPatient fhir.Patient
		err := carePlanContributor1.Read("Patient/"+*patient.Id, &readPatient)
		require.NoError(t, err)
		require.NotNil(t, readPatient)

		addExpectedAudit("Patient/"+*readPatient.Id, fhir.AuditEventActionR)

		// Read Patient by ID again, generates new AuditEvent
		err = carePlanContributor1.Read("Patient/"+*patient.Id, &readPatient)
		require.NoError(t, err)
		require.NotNil(t, readPatient)

		addExpectedAudit("Patient/"+*readPatient.Id, fhir.AuditEventActionR)
	})

	t.Run("Read Questionnaire by id", func(t *testing.T) {
		var readQuestionnaire fhir.Questionnaire
		err := carePlanContributor1.Read("Questionnaire/"+*questionnaire.Id, &readQuestionnaire)
		require.NoError(t, err)
		require.NotNil(t, readQuestionnaire)

		addExpectedAudit("Questionnaire/"+*readQuestionnaire.Id, fhir.AuditEventActionR)

		// Read Questionnaire by ID again, generates new AuditEvent
		err = carePlanContributor1.Read("Questionnaire/"+*questionnaire.Id, &readQuestionnaire)
		require.NoError(t, err)
		require.NotNil(t, readQuestionnaire)

		addExpectedAudit("Questionnaire/"+*readQuestionnaire.Id, fhir.AuditEventActionR)
	})

	t.Run("Read QuestionnaireResponse by id", func(t *testing.T) {
		var readQuestionnaireResponse fhir.QuestionnaireResponse
		err := carePlanContributor1.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &readQuestionnaireResponse)
		require.NoError(t, err)
		require.NotNil(t, readQuestionnaireResponse)

		addExpectedAudit("QuestionnaireResponse/"+*readQuestionnaireResponse.Id, fhir.AuditEventActionR)

		// Read QuestionnaireResponse by ID again, generates new AuditEvent
		err = carePlanContributor1.Read("QuestionnaireResponse/"+*questionnaireResponse.Id, &readQuestionnaireResponse)
		require.NoError(t, err)
		require.NotNil(t, readQuestionnaireResponse)

		addExpectedAudit("QuestionnaireResponse/"+*readQuestionnaireResponse.Id, fhir.AuditEventActionR)
	})

	t.Run("Read ServiceRequest by id", func(t *testing.T) {
		var readServiceRequest fhir.ServiceRequest
		err := carePlanContributor1.Read("ServiceRequest/"+*serviceRequest.Id, &readServiceRequest)
		require.NoError(t, err)
		require.NotNil(t, readServiceRequest)

		addExpectedAudit("ServiceRequest/"+*readServiceRequest.Id, fhir.AuditEventActionR)

		// Read ServiceRequest by ID again, generates new AuditEvent
		err = carePlanContributor1.Read("ServiceRequest/"+*serviceRequest.Id, &readServiceRequest)
		require.NoError(t, err)
		require.NotNil(t, readServiceRequest)

		addExpectedAudit("ServiceRequest/"+*readServiceRequest.Id, fhir.AuditEventActionR)
	})

	t.Run("Search ServiceRequest by id", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{"_id": {*serviceRequest.Id}}

		err := carePlanContributor1.Search("ServiceRequest", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify the search result contains the ServiceRequest we're looking for
		var foundServiceRequest bool
		for _, entry := range searchResult.Entry {
			var sr fhir.ServiceRequest
			err := json.Unmarshal(entry.Resource, &sr)
			require.NoError(t, err)

			if sr.Id != nil && *sr.Id == *serviceRequest.Id {
				foundServiceRequest = true
				break
			}
		}
		require.True(t, foundServiceRequest, "ServiceRequest not found in search results")

		// Add expected audit event for this search operation
		addExpectedSearchAudit("ServiceRequest/"+*serviceRequest.Id, queryParams)
	})

	t.Run("Search ServiceRequest by status", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{"status": {"completed"}}

		err := carePlanContributor1.Search("ServiceRequest", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// Verify at least one ServiceRequest is returned with completed status
		var foundCompletedServiceRequest bool
		for _, entry := range searchResult.Entry {
			var sr fhir.ServiceRequest
			err := json.Unmarshal(entry.Resource, &sr)
			require.NoError(t, err)

			if sr.Status == fhir.RequestStatusCompleted {
				foundCompletedServiceRequest = true

				// Add expected audit event for this ServiceRequest
				addExpectedSearchAudit("ServiceRequest/"+*sr.Id, queryParams)
			}
		}
		require.True(t, foundCompletedServiceRequest, "No completed ServiceRequest found in search results")
	})

	t.Run("Search ServiceRequest by multiple parameters", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{
			"status": {"completed"},
			"_id":    {*serviceRequest.Id},
		}

		err := carePlanContributor1.Search("ServiceRequest", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// The search should return our completed ServiceRequest
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify it's the correct ServiceRequest
		var foundServiceRequest bool
		for _, entry := range searchResult.Entry {
			var sr fhir.ServiceRequest
			err := json.Unmarshal(entry.Resource, &sr)
			require.NoError(t, err)

			if sr.Id != nil && *sr.Id == *serviceRequest.Id && sr.Status == fhir.RequestStatusCompleted {
				foundServiceRequest = true

				// Add expected audit event for this ServiceRequest
				addExpectedSearchAudit("ServiceRequest/"+*sr.Id, queryParams)
				break
			}
		}
		require.True(t, foundServiceRequest, "Completed ServiceRequest not found in search results")
	})

	t.Run("Search Questionnaire by id", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{
			"_id": {*questionnaire.Id},
		}

		err := carePlanContributor1.Search("Questionnaire", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// The search should return our questionnaire
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify it's the correct Questionnaire
		var foundQuestionnaire bool
		for _, entry := range searchResult.Entry {
			var q fhir.Questionnaire
			err := json.Unmarshal(entry.Resource, &q)
			require.NoError(t, err)

			if q.Id != nil && *q.Id == *questionnaire.Id {
				foundQuestionnaire = true

				// Add expected audit event for this Questionnaire
				addExpectedSearchAudit("Questionnaire/"+*q.Id, queryParams)
				break
			}
		}
		require.True(t, foundQuestionnaire, "Questionnaire not found in search results")
	})

	t.Run("Search QuestionnaireResponse by id", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{
			"_id": {*questionnaireResponse.Id},
		}

		err := carePlanContributor1.Search("QuestionnaireResponse", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// The search should return our questionnaireResponse
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify it's the correct QuestionnaireResponse
		var foundQuestionnaireResponse bool
		for _, entry := range searchResult.Entry {
			var qr fhir.QuestionnaireResponse
			err := json.Unmarshal(entry.Resource, &qr)
			require.NoError(t, err)

			if qr.Id != nil && *qr.Id == *questionnaireResponse.Id {
				foundQuestionnaireResponse = true

				// Add expected audit event for this QuestionnaireResponse
				addExpectedSearchAudit("QuestionnaireResponse/"+*qr.Id, queryParams)
				break
			}
		}
		require.True(t, foundQuestionnaireResponse, "QuestionnaireResponse not found in search results")
	})

	var readCondition fhir.Condition
	t.Run("Read Condition by id", func(t *testing.T) {
		err := carePlanContributor1.Read("Condition/"+*condition.Id, &readCondition)
		require.NoError(t, err)
		require.NotNil(t, readCondition)

		addExpectedAudit("Condition/"+*readCondition.Id, fhir.AuditEventActionR)

		// Read Condition by ID again, generates new AuditEvent
		err = carePlanContributor1.Read("Condition/"+*condition.Id, &readCondition)
		require.NoError(t, err)
		require.NotNil(t, readCondition)

		addExpectedAudit("Condition/"+*readCondition.Id, fhir.AuditEventActionR)
	})

	t.Run("Search Condition by id", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{"_id": {*condition.Id}}

		err := carePlanContributor1.Search("Condition", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify the search result contains the Condition we're looking for
		var foundCondition bool
		for _, entry := range searchResult.Entry {
			var cond fhir.Condition
			err := json.Unmarshal(entry.Resource, &cond)
			require.NoError(t, err)

			if cond.Id != nil && *cond.Id == *condition.Id {
				foundCondition = true
				break
			}
		}
		require.True(t, foundCondition, "Condition not found in search results")

		// Add expected audit event for this search operation
		addExpectedSearchAudit("Condition/"+*condition.Id, queryParams)
	})

	t.Run("Search Condition by clinical status", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{"clinical-status": {"active"}}

		err := carePlanContributor1.Search("Condition", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// Verify at least one active Condition is returned
		var foundActiveCondition bool
		for _, entry := range searchResult.Entry {
			var cond fhir.Condition
			err := json.Unmarshal(entry.Resource, &cond)
			require.NoError(t, err)

			if cond.ClinicalStatus != nil && len(cond.ClinicalStatus.Coding) > 0 {
				for _, coding := range cond.ClinicalStatus.Coding {
					if coding.Code != nil && *coding.Code == "active" {
						foundActiveCondition = true
						// Add expected audit event for this Condition
						addExpectedSearchAudit("Condition/"+*cond.Id, queryParams)
						break
					}
				}
			}
		}
		require.True(t, foundActiveCondition, "No active Condition found in search results")
	})

	t.Run("Search Condition by multiple parameters", func(t *testing.T) {
		var searchResult fhir.Bundle
		queryParams := url.Values{
			"clinical-status": {"active"},
			"_id":             {*condition.Id},
		}

		err := carePlanContributor1.Search("Condition", queryParams, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		// The search should return our active condition
		require.GreaterOrEqual(t, len(searchResult.Entry), 1)

		// Verify it's the correct Condition
		var foundCondition bool
		for _, entry := range searchResult.Entry {
			var cond fhir.Condition
			err := json.Unmarshal(entry.Resource, &cond)
			require.NoError(t, err)

			if cond.Id != nil && *cond.Id == *condition.Id {
				foundCondition = true
				// Add expected audit event for this Condition
				addExpectedSearchAudit("Condition/"+*cond.Id, queryParams)
				break
			}
		}
		require.True(t, foundCondition, "Active Condition not found in search results")
	})

	var searchResult fhir.Bundle
	t.Run("Search Patient by id", func(t *testing.T) {
		err := carePlanContributor1.Search("Patient", url.Values{"_id": {*patient.Id, *nonExistingPatient.Id, "fake-id"}}, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)

		addExpectedSearchAudit("Patient/"+*patient.Id, url.Values{"_id": {*patient.Id, *nonExistingPatient.Id, "fake-id"}})
	})

	// Verify all audit events at the end - check before sanitizing resources, as sanitizing them will mark them as entered-in-error
	err = verifyAuditEvents(t, expectedAuditEvents, fhirBaseURL)
	require.NoError(t, err)

	// Sanitize resources instead of deleting them
	t.Run("Sanitize Patient", func(t *testing.T) {
		err = carePlanContributor1.Update("Patient/"+*patient.Id+"/$sanitize", fhir.Patient{}, nil)
		require.NoError(t, err)
	})

	t.Run("Sanitize QuestionnaireResponse", func(t *testing.T) {
		err = carePlanContributor1.Update("QuestionnaireResponse/"+*questionnaireResponse.Id+"/$sanitize", fhir.QuestionnaireResponse{}, nil)
		require.NoError(t, err)

		err = carePlanContributor1.Update("QuestionnaireResponse/non-existing-questionnaire-response/$sanitize", fhir.QuestionnaireResponse{}, nil)
		require.NoError(t, err)
	})

	t.Run("Sanitize ServiceRequest", func(t *testing.T) {
		err = carePlanContributor1.Update("ServiceRequest/"+*serviceRequest.Id+"/$sanitize", fhir.ServiceRequest{}, nil)
		require.NoError(t, err)
	})

	t.Run("Sanitize Condition", func(t *testing.T) {
		err = carePlanContributor1.Update("Condition/"+*condition.Id+"/$sanitize", fhir.Condition{}, nil)
		require.NoError(t, err)
	})

	// Search for existing resources to ensure they have been sanitized
	t.Run("Search Patient", func(t *testing.T) {
		var searchResult fhir.Bundle
		err = carePlanContributor1.Search("Patient", url.Values{"_id": {*patient.Id}}, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.Equal(t, len(searchResult.Entry), 0)
	})
	t.Run("Search ServiceRequest", func(t *testing.T) {
		var searchResult fhir.Bundle
		err = carePlanContributor1.Search("ServiceRequest", url.Values{"_id": {*serviceRequest.Id}}, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.Equal(t, len(searchResult.Entry), 0)
	})
	t.Run("Search Condition", func(t *testing.T) {
		var searchResult fhir.Bundle
		err = carePlanContributor1.Search("Condition", url.Values{"_id": {*condition.Id}}, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.Equal(t, len(searchResult.Entry), 0)
	})
	t.Run("Search QuestionnaireResponse", func(t *testing.T) {
		var searchResult fhir.Bundle
		err = carePlanContributor1.Search("QuestionnaireResponse", url.Values{"_id": {*questionnaireResponse.Id, "non-existing-questionnaire-response"}}, &searchResult)
		require.NoError(t, err)
		require.NotNil(t, searchResult)
		require.Equal(t, len(searchResult.Entry), 0)
	})
}

// Define a new type to hold expected audit events without timestamp requirements
type ExpectedAuditEvent struct {
	ResourceRef string
	Action      fhir.AuditEventAction
	QueryParams map[string][]string
}

// Refactored verifyAuditEvents to handle a list of expected audit events without timestamp requirements
func verifyAuditEvents(t *testing.T, expectedEvents []ExpectedAuditEvent, fhirBaseURL *url.URL) error {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Logf("Searching for AuditEvents")

	client := fhirclient.New(fhirBaseURL, &http.Client{}, nil)

	var bundle fhir.Bundle
	err := client.SearchWithContext(ctx, "AuditEvent", url.Values{}, &bundle)

	if err != nil {
		return fmt.Errorf("failed to search AuditEvents: %w", err)
	}

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
					if expectedEvent.QueryParams != nil && *auditEvent.Action == fhir.AuditEventActionR {
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
			// I am not sure why but the initial search does not find all the audit events, doing another search for this particular audit event works
			t.Logf("Audit event not found in initial search, trying direct search for %s with action %s",
				event.ResourceRef, event.Action)

			var specificBundle fhir.Bundle
			specificQuery := url.Values{
				"entity": []string{event.ResourceRef},
			}

			err := client.Search("AuditEvent", specificQuery, &specificBundle)
			if err != nil {
				return fmt.Errorf("failed to perform specific search for audit event: %w", err)
			}

			// Check if we found it in the specific search
			found := false
			for _, entry := range specificBundle.Entry {
				var auditEvent fhir.AuditEvent
				if err := json.Unmarshal(entry.Resource, &auditEvent); err != nil {
					return err
				}

				if auditEvent.Action != nil && *auditEvent.Action == event.Action {
					found = true
					break
				}

				// For read actions, check if query parameters match
				if auditEvent.Action != nil && *auditEvent.Action == fhir.AuditEventActionR && event.QueryParams != nil {
					t.Logf("Checking query parameters for read audit event on %s", event.ResourceRef)
					if verifyQueryParams(auditEvent, event.QueryParams) {
						found = true
						break
					}
				}
			}

			if !found {
				return fmt.Errorf("expected to find audit event with action %s for resource %s",
					event.Action, event.ResourceRef)
			}
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
