#!/bin/sh

echo "    Fetching $1/fhir/metadata"
# Wait for FHIR server to be ready
until $(curl --output /dev/null --silent --fail $1/fhir/metadata); do
  echo "    Waiting for FHIR server to be ready..."
  sleep 5
done

#TODO: Remove Questionnaire creation @ INT-211
echo "    Creating enrollment Questionnaire"
curl --silent -X PUT "$1/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.18--20240704100750" \
    -H "Content-Type: application/fhir+json" \
    -d '{
  "resourceType": "Questionnaire",
  "id": "2.16.840.1.113883.2.4.3.11.60.909.26.18--20240704100750",
  "meta": {
    "profile": [
      "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-pop-exp",
      "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-render"
    ]
  },
  "status": "draft",
  "title": "Orderformulier Zorg bij jou",
  "subjectType": ["Patient"],
  "extension": [
    {
      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext",
      "extension": [
        {
          "url": "name",
          "valueCoding": {
            "system": "http://hl7.org/fhir/uv/sdc/CodeSystem/launchContext",
            "code": "patient"
          }
        },
        {
          "url": "type",
          "valueCode": "Patient"
        },
        {
          "url": "description",
          "valueString": "The patient that is to be used to pre-populate the form"
        }
      ]
    },
    {
      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext",
      "extension": [
        {
          "url": "name",
          "valueCoding": {
            "system": "http://hl7.org/fhir/uv/sdc/CodeSystem/launchContext",
            "code": "user"
          }
        },
        {
          "url": "type",
          "valueCode": "Practitioner"
        },
        {
          "url": "description",
          "valueString": "The practitioner user that is to be used to pre-populate the form"
        }
      ]
    },
    {
      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-launchContext",
      "extension": [
        {
          "url": "name",
          "valueCoding": {
            "system": "http://hl7.org/fhir/uv/sdc/CodeSystem/launchContext",
            "code": "encounter"
          }
        },
        {
          "url": "type",
          "valueCode": "Encounter"
        },
        {
          "url": "description",
          "valueString": "The encounter that is to be used to pre-populate the form"
        }
      ]
    }
  ],
  "item": [
    {
      "extension": [
        {
          "url": "http://hl7.org/fhir/StructureDefinition/questionnaire-itemControl",
          "valueCodeableConcept": {
            "coding": [
              {
                "system": "http://hl7.org/fhir/questionnaire-item-control",
                "code": "tab-container"
              }
            ]
          }
        }
      ],
      "linkId": "tab-container",
      "type": "group",
      "item": [
        {
          "linkId": "inclusiecriteria-tab",
          "text": "Inclusiecriteria",
          "type": "group",
          "item": [
            {
              "linkId": "inclusiecriteria",
              "type": "group",
              "required": true,
              "item": [
                {
                  "linkId": "smartphone",
                  "text": "Patiënt heeft een smartphone",
                  "type": "boolean",
                  "required": true
                },
                {
                  "linkId": "email-smartphone",
                  "text": "Patiënt of mantelzorger leest e-mail op smartphone",
                  "type": "boolean",
                  "required": true
                },
                {
                  "linkId": "install-apps",
                  "text": "Patiënt of mantelzorger kan apps installeren op smartphone",
                  "type": "boolean",
                  "required": true
                },
                {
                  "linkId": "dutch-language",
                  "text": "Patiënt of mantelzorger is de nederlandse taal machtig",
                  "type": "boolean",
                  "required": true
                },
                {
                  "linkId": "equipment",
                  "text": "Patiënt beschikt over een weegschaal en bloeddrukmeter (of gaat deze aanschaffen)",
                  "type": "boolean",
                  "required": true
                }
              ]
            }
          ]
        },
        {
          "linkId": "patient-practitioner-info-tab",
          "text": "Order",
          "type": "group",
          "item": [
            {
              "linkId": "patient-details",
              "text": "Patiëntgegevens",
              "type": "group",
              "required": true,
              "item": [
                {
                  "linkId": "first-name",
                  "text": "Voornaam",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.name.given.first()"
                      }
                    }
                  ]
                },
                {
                  "linkId": "middle-name",
                  "text": "Tussenvoegsel (optioneel)",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.name.given.last()"
                      }
                    }
                  ]
                },
                {
                  "linkId": "last-name",
                  "text": "Achternaam",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.name.family"
                      }
                    }
                  ]
                },
                {
                  "linkId": "bsn",
                  "text": "BSN",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.identifier.where(system='\'http://fhir.nl/fhir/NamingSystem/bsn\'').value"
                      }
                    }
                  ]
                },
                {
                  "linkId": "birthdate",
                  "text": "Geboortedatum",
                  "type": "date",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.birthDate"
                      }
                    }
                  ]
                },
                {
                  "linkId": "gender",
                  "text": "Geslacht bij geboorte",
                  "type": "choice",
                  "required": true,
                  "answerValueSet": "http://hl7.org/fhir/ValueSet/administrative-gender",
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.gender"
                      }
                    }
                  ]
                },
                {
                  "linkId": "email",
                  "text": "E-mailadres",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.telecom.where(system='\'email\'').value"
                      }
                    }
                  ]
                },
                {
                  "linkId": "phone-number",
                  "text": "Telefoonnummer",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%patient.telecom.where(system='\'phone\'').value"
                      }
                    }
                  ]
                },
                {
                  "linkId": "condition",
                  "text": "Aandoening",
                  "type": "open-choice",
                  "required": true,
                  "answerValueSet": "http://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.1--20171231000000"
                },
                {
                  "linkId": "protocol",
                  "text": "Protocol",
                  "type": "choice",
                  "required": true,
                  "answerOption": [
                    {
                      "valueCoding": {
                        "code": "S",
                        "display": "Small"
                      }
                    },
                    {
                      "valueCoding": {
                        "code": "M",
                        "display": "Medium"
                      }
                    },
                    {
                      "valueCoding": {
                        "code": "L",
                        "display": "Large"
                      }
                    },
                    {
                      "valueCoding": {
                        "code": "XL",
                        "display": "Extra Large"
                      }
                    }
                  ]
                }
              ]
            },
            {
              "linkId": "requester-details",
              "text": "Aanvragende behandelaar",
              "type": "group",
              "required": true,
              "item": [
                {
                  "linkId": "requester-first-name",
                  "text": "Voornaam",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%user.name.given.first()"
                      }
                    }
                  ]
                },
                {
                  "linkId": "requester-last-name",
                  "text": "Achternaam",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%user.name.family"
                      }
                    }
                  ]
                },
                {
                  "linkId": "requester-email",
                  "text": "E-mailadres",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%user.telecom.where(system='\'email\'').value"
                      }
                    }
                  ]
                },
                {
                  "linkId": "requester-phone-number",
                  "text": "Telefoonnummer",
                  "type": "string",
                  "required": true,
                  "extension": [
                    {
                      "url": "http://hl7.org/fhir/uv/sdc/StructureDefinition/sdc-questionnaire-initialExpression",
                      "valueExpression": {
                        "language": "text/fhirpath",
                        "expression": "%user.telecom.where(system='\'phone\'').value"
                      }
                    }
                  ]
                }
              ]
            },
            {
              "linkId": "comments",
              "text": "Opmerkingen",
              "type": "text",
              "required": true
            }
          ]
        }
      ]
    }
  ]
}'