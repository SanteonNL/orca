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
        "lastUpdated": "2024-07-09T13:31:30Z",
        "source": "http://decor.nictiz.nl/fhir/4.0/sansa-",
        "tag": [
            {
                "system": "http://hl7.org/fhir/FHIR-version",
                "code": "4.0.1"
            }
        ]
    },
    "language": "nl-NL",
    "url": "http://decor.nictiz.nl/fhir/Questionnaire/2.16.840.1.113883.2.4.3.11.60.909.26.18--20240704100750",
    "identifier": [
        {
            "system": "urn:ietf:rfc:3986",
            "value": "urn:oid:2.16.840.1.113883.2.4.3.11.60.909.26.28"
        }
    ],
    "name": "PerformOnboarding",
    "title": "PerformOnboarding",
    "status": "draft",
    "experimental": false,
    "date": "2024-07-09T13:31:30Z",
    "publisher": "Santeon",
    "effectivePeriod": {
        "start": "2024-07-09T13:31:30Z"
    },
    "item": [
        {
            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2350",
            "text": "ServiceRequest",
            "type": "group",
            "required": true,
            "repeats": false,
            "readOnly": false,
            "item": [
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2526",
                    "text": "Patient",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Patient"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2560",
                            "text": "Identificatienummer",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "PatientIdentificationNumber"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "string",
                            "required": true,
                            "repeats": false,
                            "readOnly": false
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2566",
                    "text": "Zorgaanbieder",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "HealthcareProvider"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2567",
                            "text": "ZorgaanbiederIdentificatienummer",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "HealthcareProviderIdentificationNumber"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "string",
                            "required": true,
                            "repeats": false,
                            "readOnly": false
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2595",
                    "text": "Zorgaanbieder",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "HealthcareProvider"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2596",
                            "text": "ZorgaanbiederIdentificatienummer",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "HealthcareProviderIdentificationNumber"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "string",
                            "required": true,
                            "repeats": false,
                            "readOnly": false
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2624",
                    "text": "Verrichting",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Procedure"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2627",
                            "text": "VerrichtingType",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ProcedureType"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "open-choice",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "answerValueSet": "https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.14.1.8--20200901000000 https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.14.1.7--20200901000000 https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.14.1.6--20200901000000 https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.14.1.5--20200901000000 https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.14.1.2--20200901000000"
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2642",
                    "text": "Probleem",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Problem"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2644",
                            "text": "ProbleemNaam",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ProblemName"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "open-choice",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "answerValueSet": "https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000"
                        }
                    ]
                }
            ]
        },
        {
            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2207",
            "text": "Inclusiecriteria",
            "type": "group",
            "required": true,
            "repeats": false,
            "readOnly": false,
            "item": [
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2208",
                    "text": "Patient heeft smartphone",
                    "type": "boolean",
                    "required": true,
                    "repeats": false,
                    "readOnly": false
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2209",
                    "text": "Patient of mantelzorger leest e-mail op smartphone",
                    "type": "boolean",
                    "required": true,
                    "repeats": false,
                    "readOnly": false
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2210",
                    "text": "Patient of mantelzorger kan apps installeren op smartphone",
                    "type": "boolean",
                    "required": true,
                    "repeats": false,
                    "readOnly": false
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2211",
                    "text": "Patient of mantelzorger is Nederlandse taal machtig",
                    "type": "boolean",
                    "required": true,
                    "repeats": false,
                    "readOnly": false
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2212",
                    "text": "Patient beschikt over een weegschaal of bloeddrukmeter (of gaat deze aanschaffen)",
                    "type": "boolean",
                    "required": true,
                    "repeats": false,
                    "readOnly": false
                }
            ]
        },
        {
            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2312",
            "text": "Order",
            "type": "group",
            "required": true,
            "repeats": true,
            "readOnly": false,
            "item": [
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2232",
                    "text": "Patient",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Patient"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2233",
                            "text": "Naamgegevens",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "NameInformation"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "group",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "item": [
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2234",
                                    "text": "Voornamen",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "FirstNames"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "string",
                                    "required": true,
                                    "repeats": true,
                                    "readOnly": false
                                },
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2238",
                                    "text": "Geslachtsnaam",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "LastName"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": true,
                                    "readOnly": false,
                                    "item": [
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2239",
                                            "text": "Voorvoegsels",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "Prefix"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": false,
                                            "repeats": false,
                                            "readOnly": false
                                        },
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2240",
                                            "text": "Achternaam",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "LastName"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2257",
                            "text": "Contactgegevens",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ContactInformation"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "group",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "item": [
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2258",
                                    "text": "Telefoonnummers",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "TelephoneNumbers"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": false,
                                    "readOnly": false,
                                    "item": [
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2259",
                                            "text": "Telefoonnummer",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "TelephoneNumber"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        }
                                    ]
                                },
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2263",
                                    "text": "EmailAdressen",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "EmailAddresses"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": false,
                                    "readOnly": false,
                                    "item": [
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2264",
                                            "text": "EmailAdres",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "EmailAddress"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2266",
                            "text": "Identificatienummer",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "PatientIdentificationNumber"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "string",
                            "required": true,
                            "repeats": false,
                            "readOnly": false
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2267",
                            "text": "Geboortedatum",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "DateOfBirth"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "dateTime",
                            "required": true,
                            "repeats": false,
                            "readOnly": false
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2268",
                            "text": "Geslacht",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "Gender"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "choice",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "answerOption": [
                                {
                                    "valueCoding": {
                                        "system": "http://terminology.hl7.org/CodeSystem/v3-AdministrativeGender",
                                        "code": "UN",
                                        "display": "Undifferentiated"
                                    }
                                },
                                {
                                    "valueCoding": {
                                        "system": "http://terminology.hl7.org/CodeSystem/v3-AdministrativeGender",
                                        "code": "M",
                                        "display": "Male"
                                    }
                                },
                                {
                                    "valueCoding": {
                                        "system": "http://terminology.hl7.org/CodeSystem/v3-AdministrativeGender",
                                        "code": "F",
                                        "display": "Female"
                                    }
                                },
                                {
                                    "valueCoding": {
                                        "system": "http://terminology.hl7.org/CodeSystem/v3-NullFlavor",
                                        "code": "UNK",
                                        "display": "Unknown"
                                    }
                                }
                            ]
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2313",
                    "text": "Probleem",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Problem"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2315",
                            "text": "ProbleemNaam",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ProblemName"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "open-choice",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "answerValueSet": "https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000"
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2654",
                    "text": "Probleem",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "Problem"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": false,
                    "repeats": true,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2656",
                            "text": "ProbleemNaam",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ProblemName"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "open-choice",
                            "required": false,
                            "repeats": true,
                            "readOnly": false,
                            "answerValueSet": "https://decor.nictiz.nl/fhir/ValueSet/2.16.840.1.113883.2.4.3.11.60.40.2.5.1.3--20200901000000"
                        }
                    ]
                },
                {
                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2272",
                    "text": "Zorgverlener",
                    "_text": {
                        "extension": [
                            {
                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                "extension": [
                                    {
                                        "url": "lang",
                                        "valueCode": "en-US"
                                    },
                                    {
                                        "url": "content",
                                        "valueString": "HealthProfessional"
                                    }
                                ]
                            }
                        ]
                    },
                    "type": "group",
                    "required": true,
                    "repeats": false,
                    "readOnly": false,
                    "item": [
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2274",
                            "text": "Naamgegevens",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "NameInformation"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "group",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "item": [
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2275",
                                    "text": "Voornamen",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "FirstNames"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "string",
                                    "required": true,
                                    "repeats": true,
                                    "readOnly": false
                                },
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2279",
                                    "text": "Geslachtsnaam",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "LastName"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": true,
                                    "readOnly": false,
                                    "item": [
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2280",
                                            "text": "Voorvoegsels",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "Prefix"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        },
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2281",
                                            "text": "Achternaam",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "LastName"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2300",
                            "text": "Contactgegevens",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "ContactInformation"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "group",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "item": [
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2306",
                                    "text": "EmailAdressen",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "EmailAddresses"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": false,
                                    "readOnly": false,
                                    "item": [
                                        {
                                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2307",
                                            "text": "EmailAdres",
                                            "_text": {
                                                "extension": [
                                                    {
                                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                        "extension": [
                                                            {
                                                                "url": "lang",
                                                                "valueCode": "en-US"
                                                            },
                                                            {
                                                                "url": "content",
                                                                "valueString": "EmailAddress"
                                                            }
                                                        ]
                                                    }
                                                ]
                                            },
                                            "type": "string",
                                            "required": true,
                                            "repeats": false,
                                            "readOnly": false
                                        }
                                    ]
                                }
                            ]
                        },
                        {
                            "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2309",
                            "text": "Zorgaanbieder",
                            "_text": {
                                "extension": [
                                    {
                                        "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                        "extension": [
                                            {
                                                "url": "lang",
                                                "valueCode": "en-US"
                                            },
                                            {
                                                "url": "content",
                                                "valueString": "HealthcareProvider"
                                            }
                                        ]
                                    }
                                ]
                            },
                            "type": "group",
                            "required": true,
                            "repeats": false,
                            "readOnly": false,
                            "item": [
                                {
                                    "linkId": "2.16.840.1.113883.2.4.3.11.60.909.2.2.2310",
                                    "text": "Zorgaanbieder",
                                    "_text": {
                                        "extension": [
                                            {
                                                "url": "http://hl7.org/fhir/StructureDefinition/translation",
                                                "extension": [
                                                    {
                                                        "url": "lang",
                                                        "valueCode": "en-US"
                                                    },
                                                    {
                                                        "url": "content",
                                                        "valueString": "HealthcareProvider"
                                                    }
                                                ]
                                            }
                                        ]
                                    },
                                    "type": "group",
                                    "required": true,
                                    "repeats": false,
                                    "readOnly": false
                                }
                            ]
                        }
                    ]
                }
            ]
        }
    ]
}'