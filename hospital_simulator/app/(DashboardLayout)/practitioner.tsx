import {DefaultAzureCredential} from '@azure/identity';

async function authenticateWithDefaultCredential() {
    const fhirUrl = process.env.FHIR_BASE_URL || '';
    if (fhirUrl.includes('localhost') || fhirUrl.includes('fhirstore')) {
        return null;
    }

    try {
        const credential = new DefaultAzureCredential();
        const tokenResponse = await credential.getToken(`${fhirUrl}/.default`);

        return tokenResponse.token;
    } catch (error) {
        console.error('authenticateWithDefaultCredential authentication failed:', error);
        throw error;
    }
}

export default async function CreatePractitioner() {
    // Create the following resource:
    const bundle = {
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:practitioner-1",
                "resource": {
                    "resourceType": "Practitioner",
                    "id": "practitioner-1",
                    "identifier": [
                        {
                            "system": "http://fhir.nl/fhir/NamingSystem/uzi",
                            "value": "uzi-001"
                        }
                    ],
                    "name": [
                        {
                            "family": "Hans",
                            "given": ["Visser"],
                            "prefix": ["dr."]
                        }
                    ],
                    "telecom": [
                        {
                            "system": "phone",
                            "value": "987-654-3210",
                            "use": "work"
                        },
                        {
                            "system": "email",
                            "value": "practitioner@example.com",
                            "use": "work"
                        }
                    ],
                    "address": [
                        {
                            "use": "work",
                            "line": "GPStreet 17",
                            "city": "Health City",
                            "postalCode": "1234 AA",
                            "country": "NL"
                        }
                    ]
                },
                "request": {
                    "method": "POST",
                    "url": "Practitioner",
                    "ifNoneExist": "identifier=http://fhir.nl/fhir/NamingSystem/uzi|uzi-001"
                }
            },
            {
                "fullUrl": "urn:uuid:practitionerrole-1",
                "resource": {
                    "resourceType": "PractitionerRole",
                    "id": "practitionerrole-1",
                    "identifier": [
                        {
                            "system": "http://fhir.nl/fhir/NamingSystem/uzi",
                            "value": "uzi-001"
                        }
                    ],
                    "practitioner": {
                        "type": "Practitioner",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/uzi",
                            "value": "uzi-001"
                        }
                    },
                    "code": [
                        {
                            "coding": [
                                {
                                    "system": "http://terminology.hl7.org/CodeSystem/practitioner-role",
                                    "code": "doctor"
                                }
                            ]
                        }
                    ]
                },
                "request": {
                    "method": "POST",
                    "url": "PractitionerRole",
                    "ifNoneExist": "identifier=http://fhir.nl/fhir/NamingSystem/uzi|uzi-001"
                }
            }
        ]
    };


    const requestURL = `${process.env.FHIR_BASE_URL}/`;
    const token = await authenticateWithDefaultCredential();
    const headers: HeadersInit = {
        "Content-Type": "application/json"
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    console.log(`Sending Practitioner and PractitionerRole creation request to: ${requestURL}`);
    const response = await fetch(requestURL, {
        method: "POST",
        cache: 'no-store',
        headers: headers,
        body: JSON.stringify(bundle)
    });
    if (!response.ok) {
        throw new Error('Failed to create Practitioner and PractitionerRole: ' + response.statusText);
    }
}