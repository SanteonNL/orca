#!/bin/sh

echo "    Fetching $1/fhir/metadata"
# Wait for FHIR server to be ready
until $(curl --output /dev/null --silent --fail $1/fhir/metadata); do
  echo "    Waiting for FHIR server to be ready..."
  sleep 5
done

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
                  "answerValueSet": "'$1/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000'"
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

curl -X PUT "$1/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000" \
    -H "Content-Type: application/fhir+json" \
    -d '{
    "resourceType": "ValueSet",
    "id": "2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000",
    "meta": {
        "profile": [
            "http://hl7.org/fhir/StructureDefinition/shareablevalueset"
        ]
    },
    "extension": [
        {
            "url": "http://hl7.org/fhir/StructureDefinition/resource-effectivePeriod",
            "valuePeriod": {
                "start": "2020-09-01T00:00:00Z"
            }
        }
    ],
    "url": "'$1/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000'",
    "identifier": [
        {
            "use": "official",
            "system": "urn:ietf:rfc:3986",
            "value": "urn:oid:2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3"
        }
    ],
    "version": "2020-09-01T00:00:00",
    "name": "ProbleemNaamCodelijst",
    "title": "ProbleemNaamCodelijst",
    "status": "active",
    "experimental": false,
    "publisher": "Registratie aan de bron",
    "contact": [
        {
            "name": "Registratie aan de bron",
            "telecom": [
                {
                    "system": "url",
                    "value": "https://www.registratieaandebron.nl"
                },
                {
                    "system": "url",
                    "value": "https://www.zibs.nl"
                }
            ]
        }
    ],
    "description": "* OMAHA [DEPRECATED] - Alle waarden [DEPRECATED]\n* G-Standaard Contra Indicaties (Tabel 40) [DEPRECATED] - Alle waarden [DEPRECATED]\n* SNOMED CT - SNOMED CT: ^31000147101 | DHD Diagnosethesaurus-referentieset (was: DHD Diagnosetheauruscodes met OID 2.16.840.1.113883.2.4.3.120.5.1. Deze set is echter niet direct bedoeld voor uitwisseling. Zie onder andere [ZIB-1233](https://bits.nictiz.nl/browse/ZIB-1233)\n* ICD-10, dutch translation - Alle waarden\n* SNOMED CT - SNOMED CT: ^11721000146100 | RefSet Patiëntproblemen V&VN\n* NANDA-I - Alle waarden [DEPRECATED]\n* ICF - Alle waarden\n* ICPC-1 NL - Alle waarden\n* DSM-IV - Alle waarden\n* DSM-5 - Alle waarden\n* GGZ Diagnoselijst - Alle waarden\n\n                 \n* 2021-08-18 [ZIB-1477](https://bits.nictiz.nl/browse/ZIB-1477): Probleem-v4.4. Aanpassing codelijst ProbleemNaamCodelijst. Bij het concept ProbleemNaam bevat de ProbleemNaamCodelijst een een entry voor NANDA-I als een toegestaan codesysteem om een probleem te coderen. Deze keuzemogelijkheid is vervallen.",
    "immutable": false,
    "copyright": "This artefact includes content from SNOMED Clinical Terms® (SNOMED CT®) which is copyright of the International Health Terminology Standards Development Organisation (IHTSDO). Implementers of these artefacts must have the appropriate SNOMED CT Affiliate license - for more information contact http://www.snomed.org/snomed-ct/getsnomed-ct or info@snomed.org.",
    "compose": {
        "include": [
            {
                "system": "urn:oid:2.16.840.1.113883.6.98"
            },
            {
                "system": "urn:oid:2.16.840.1.113883.2.4.4.1.902.40"
            },
            {
                "system": "http://hl7.org/fhir/sid/icd-10-nl"
            },
            {
                "system": "http://www.nanda.org/"
            },
            {
                "system": "http://hl7.org/fhir/sid/icf-nl"
            },
            {
                "system": "http://hl7.org/fhir/sid/icpc-1-nl"
            },
            {
                "system": "urn:oid:2.16.840.1.113883.6.126"
            },
            {
                "system": "http://hl7.org/fhir/sid/dsm5"
            },
            {
                "system": "urn:oid:2.16.840.1.113883.3.3210.14.2.2.35"
            },
            {
                "system": "http://snomed.info/sct",
                "filter": [
                    {
                        "property": "concept",
                        "op": "in",
                        "value": "31000147101"
                    }
                ]
            },
            {
                "system": "http://snomed.info/sct",
                "filter": [
                    {
                        "property": "concept",
                        "op": "in",
                        "value": "11721000146100"
                    }
                ]
            }
        ]
    }
}'