package careplanservice

import "github.com/google/uuid"

func (s *Service) getQuestionnaireByUrl(url string) map[string]interface{} {
	switch url {
	case "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017":
		return s.getHardCodedHomeMonitoringPIIQuestionnaire()
	case "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017":
		return s.getHardCodedHomeMonitoringQuestionnaire()
	default:
		return nil
	}
}

func (s *Service) getHardCodedHomeMonitoringPIIQuestionnaire() map[string]interface{} {
	return map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Questionnaire",
		"meta": map[string]interface{}{
			"lastUpdated": "2024-09-02T13:40:17Z",
			"source":      "http://decor.nictiz.nl/fhir/4.0/sansa-",
			"profile": []string{
				"http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-pop-exp",
				"http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-render",
			},
			"tag": []map[string]interface{}{
				{
					"system": "http://hl7.org/fhir/FHIR-version",
					"code":   "4.0.1",
				},
			},
		},
		"language": "nl-NL",
		"text": map[string]interface{}{
			"status": "generated",
			"div":    "<div xmlns=\"http://www.w3.org/1999/xhtml\" xml:lang=\"nl-NL\" lang=\"nl-NL\"><p>Generated Narrative: Questionnaire cps-questionnaire-patient-details</p></div>",
		},
		"extension": []map[string]interface{}{
			{
				"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext",
				"extension": []map[string]interface{}{
					{
						"url": "name",
						"valueCoding": map[string]interface{}{
							"system": "http://hl7.org/fhir/uv/sdc/CodeSystem/launchContext",
							"code":   "patient",
						},
					},
					{
						"url":       "type",
						"valueCode": "Patient",
					},
					{
						"url":         "description",
						"valueString": "The patient that is to be used to pre-populate the form",
					},
				},
			},
		},
		"url": "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-2--20240902134017",
		"identifier": []map[string]interface{}{
			{
				"system": "urn:oid:2.16.840.1.113883.2.4.3.11.60.909.26.34",
				"value":  "2",
			},
		},
		"name":         "patient contactdetails",
		"title":        "patient contactdetails",
		"status":       "draft",
		"experimental": false,
		"date":         "2024-09-02T13:40:17Z",
		"publisher":    "Medical Service Centre",
		"effectivePeriod": map[string]interface{}{
			"start": "2024-09-02T13:40:17Z",
		},
		"item": []map[string]interface{}{
			{
				"linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2233",
				"text":   "Naamgegevens",
				"_text": map[string]interface{}{
					"extension": []map[string]interface{}{
						{
							"extension": []map[string]interface{}{
								{
									"url":       "lang",
									"valueCode": "en-US",
								},
								{
									"url":         "content",
									"valueString": "NameInformation",
								},
							},
							"url": "http://hl7.org/fhir/StructureDefinition/translation",
						},
					},
				},
				"type":     "group",
				"required": true,
				"repeats":  false,
				"readOnly": false,
				"item": []map[string]interface{}{
					{
						"extension": []map[string]interface{}{
							{
								"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
								"valueExpression": map[string]interface{}{
									"language":   "text/fhirpath",
									"expression": "%patient.name.first().given.first()",
								},
							},
						},
						"linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2234",
						"text":   "Voornamen",
						"_text": map[string]interface{}{
							"extension": []map[string]interface{}{
								{
									"extension": []map[string]interface{}{
										{
											"url":       "lang",
											"valueCode": "en-US",
										},
										{
											"url":         "content",
											"valueString": "FirstNames",
										},
									},
									"url": "http://hl7.org/fhir/StructureDefinition/translation",
								},
							},
						},
						"type":     "string",
						"required": true,
						"repeats":  true,
						"readOnly": false,
					},
					{
						"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2238",
						"text":     "Geslachtsnaam",
						"type":     "group",
						"required": true,
						"readOnly": false,
						"item": []map[string]interface{}{
							{
								"extension": []map[string]interface{}{
									{
										"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
										"valueExpression": map[string]interface{}{
											"language":   "text/fhirpath",
											"expression": "%patient.name.given.last()",
										},
									},
								},
								"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2239",
								"text":     "Voorvoegsels",
								"type":     "string",
								"required": false,
								"repeats":  false,
								"readOnly": false,
							},
							{
								"extension": []map[string]interface{}{
									{
										"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
										"valueExpression": map[string]interface{}{
											"language":   "text/fhirpath",
											"expression": "%patient.name.family",
										},
									},
								},
								"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2240",
								"text":     "Achternaam",
								"type":     "string",
								"required": true,
								"repeats":  false,
								"readOnly": false,
							},
						},
					},
				},
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2257",
				"text":     "Contactgegevens",
				"type":     "group",
				"required": true,
				"repeats":  false,
				"readOnly": false,
				"item": []map[string]interface{}{
					{
						"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2258",
						"text":     "Telefoonnummers",
						"type":     "group",
						"required": true,
						"repeats":  false,
						"readOnly": false,
						"item": []map[string]interface{}{
							{
								"extension": []map[string]interface{}{
									{
										"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
										"valueExpression": map[string]interface{}{
											"language":   "text/fhirpath",
											"expression": "%patient.telecom.where(system='phone').value",
										},
									},
								},
								"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2259",
								"text":     "Telefoonnummer",
								"type":     "string",
								"required": true,
								"repeats":  false,
								"readOnly": false,
							},
						},
					},
					{
						"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2263",
						"text":     "EmailAdressen",
						"type":     "group",
						"required": true,
						"repeats":  false,
						"readOnly": false,
						"item": []map[string]interface{}{
							{
								"extension": []map[string]interface{}{
									{
										"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
										"valueExpression": map[string]interface{}{
											"language":   "text/fhirpath",
											"expression": "%patient.telecom.where(system='email').value",
										},
									},
								},
								"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2264",
								"text":     "EmailAdres",
								"type":     "string",
								"required": true,
								"repeats":  false,
								"readOnly": false,
							},
						},
					},
				},
			},
			{
				"extension": []map[string]interface{}{
					{
						"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
						"valueExpression": map[string]interface{}{
							"language":   "text/fhirpath",
							"expression": "%patient.identifier.where(system='http://fhir.nl/fhir/NamingSystem/bsn').value",
						},
					},
				},
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2266",
				"text":     "Burgerservicenummer (OID: 2.16.840.1.113883.2.4.6.3)",
				"type":     "string",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"extension": []map[string]interface{}{
					{
						"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
						"valueExpression": map[string]interface{}{
							"language":   "text/fhirpath",
							"expression": "%patient.birthDate",
						},
					},
				},
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2267",
				"text":     "Geboortedatum",
				"type":     "dateTime",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"extension": []map[string]interface{}{
					{
						"url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
						"valueExpression": map[string]interface{}{
							"language":   "text/fhirpath",
							"expression": "%patient.gender",
						},
					},
				},
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2268",
				"text":     "Geslacht",
				"type":     "choice",
				"required": true,
				"repeats":  false,
				"readOnly": false,
				"answerOption": []map[string]interface{}{
					{
						"valueCoding": map[string]interface{}{
							"system":  "http://hl7.org/fhir/administrative-gender",
							"code":    "other",
							"display": "Other",
						},
					},
					{
						"valueCoding": map[string]interface{}{
							"system":  "http://hl7.org/fhir/administrative-gender",
							"code":    "male",
							"display": "Male",
						},
					},
					{
						"valueCoding": map[string]interface{}{
							"system":  "http://hl7.org/fhir/administrative-gender",
							"code":    "female",
							"display": "Female",
						},
					},
					{
						"valueCoding": map[string]interface{}{
							"system":  "http://hl7.org/fhir/administrative-gender",
							"code":    "unknown",
							"display": "Unknown",
						},
					},
				},
			},
		},
	}
}

func (s *Service) getHardCodedHomeMonitoringQuestionnaire() map[string]interface{} {
	return map[string]interface{}{
		"id":           uuid.NewString(),
		"resourceType": "Questionnaire",
		"meta": map[string]interface{}{
			"source": "http://decor.nictiz.nl/fhir/4.0/sansa-",
			"tag": []map[string]interface{}{
				{
					"system": "http://hl7.org/fhir/FHIR-version",
					"code":   "4.0.1",
				},
			},
		},
		"language": "nl-NL",
		"url":      "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.34-1--20240902134017",
		"identifier": []map[string]interface{}{
			{
				"system": "urn:ietf:rfc:3986",
				"value":  "urn:oid:2.16.840.1.113883.2.4.3.11.60.909.26.34-1",
			},
		},
		"name":         "Telemonitoring - enrollment criteria",
		"title":        "Telemonitoring - enrollment criteria",
		"status":       "active",
		"experimental": false,
		"date":         "2024-09-02T13:40:17Z",
		"publisher":    "Medical Service Centre",
		"effectivePeriod": map[string]interface{}{
			"start": "2024-09-02T13:40:17Z",
		},
		"item": []map[string]interface{}{
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2208",
				"text":     "Patient heeft smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2209",
				"text":     "Patient of mantelzorger leest e-mail op smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2210",
				"text":     "Patient of mantelzorger kan apps installeren op smartphone",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2211",
				"text":     "Patient of mantelzorger is Nederlandse taal machtig",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
			{
				"linkId":   "2.16.840.1.113883.2.4.3.11.60.909.2.2.2212",
				"text":     "Patient beschikt over een weegschaal of bloeddrukmeter (of gaat deze aanschaffen)",
				"type":     "boolean",
				"required": true,
				"repeats":  false,
				"readOnly": false,
			},
		},
	}
}
