"use client"
import * as React from 'react';
import Button from '@mui/material/Button';
import TextField from '@mui/material/TextField';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import { IconPlus } from '@tabler/icons-react';
import { Alert, Grid, MenuItem, Select } from '@mui/material';
import { useState } from 'react';
import { useRouter } from 'next/navigation';

const CreateServiceRequestDialog: React.FC = () => {
    const [open, setOpen] = React.useState(false);
    const [patientFirstName, setPatientFirstName] = useState<string>()
    const [patientLastName, setPatientLastName] = useState<string>()
    const [error, setError] = useState<string>()
    const router = useRouter()

    const handleClickOpen = () => {
        setOpen(true);
    };

    const handleClose = () => {
        setOpen(false);
    };

    const createServiceRequest = async () => {
        const bundle = createServiceRequestBundle(patientFirstName || "John", patientLastName || "Doe")

        const resp = await fetch(`${process.env.NEXT_PUBLIC_BASE_PATH || ''}/api/fhir`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify(bundle)
        })

        if (!resp.ok) {
            setError(`[${resp.status}] ${await resp.text()}`)
            return
        }

        handleClose()

        router.refresh()
    }

    return (

        <React.Fragment>
            <Button sx={{ position: 'absolute', top: '10px', right: '10px' }} variant="contained" onClick={handleClickOpen}>
                <IconPlus />
            </Button>
            <Dialog
                open={open}
                onClose={handleClose}
                aria-labelledby="alert-dialog-title"
                aria-describedby="alert-dialog-description"
            >
                <DialogTitle id="alert-dialog-title">New ServiceRequest</DialogTitle>
                <DialogContent>
                    <Grid container spacing={2}>
                        {error && (
                            <Grid item xs={12}>
                                <Alert severity="error">Something went wrong: {error}</Alert>
                            </Grid>
                        )}
                        <Grid item xs={12}>
                            <DialogContentText>
                                Create a new ServiceRequest for a new Patient. For demo purposes, we can only create <i>Telemonitoring</i> ServiceRequests.
                            </DialogContentText>
                        </Grid>
                        <Grid item xs={12} md={6}>
                            <TextField
                                autoFocus
                                required
                                label="Patient first name"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientFirstName(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid item xs={12} md={6}>
                            <TextField
                                autoFocus
                                required
                                label="Patient family name"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientLastName(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid item xs={12}>
                            <Select fullWidth value="tele-monitoring">
                                <MenuItem value="tele-monitoring">Telemonitoring</MenuItem>
                            </Select>
                        </Grid>
                    </Grid>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleClose}>Cancel</Button>
                    <Button onClick={createServiceRequest} autoFocus>
                        Create
                    </Button>
                </DialogActions>
            </Dialog>
        </React.Fragment>
    );
}

export default CreateServiceRequestDialog

function createServiceRequestBundle(firstName: string, lastName: string) {

    const patientBsn = Date.now();

    return {
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:patient-1",
                "resource": {
                    "resourceType": "Patient",
                    "identifier": [
                        {
                            "use": "usual",
                            "system": "http://fhir.nl/fhir/NamingSystem/bsn",
                            "value": patientBsn
                        }
                    ],
                    "name": [
                        {
                            "family": lastName,
                            "given": [
                                firstName,
                                "MiddleName"
                            ],
                            "text": `${lastName}, ${firstName}`
                        }
                    ],
                    "gender": "male",
                    "birthDate": "1980-01-01",
                    "address": [
                        {
                            "use": "home",
                            "type": "postal",
                            "line": ["123 Main Street"],
                            "city": "Hometown",
                            "state": "State",
                            "postalCode": "12345",
                            "country": "Country"
                        }
                    ],
                    "telecom": [
                        {
                            "system": "phone",
                            "value": "123-456-7890",
                            "use": "home"
                        },
                        {
                            "system": "email",
                            "value": "patient@example.com",
                            "use": "home"
                        }
                    ]
                },
                "request": {
                    "method": "POST",
                    "url": "Patient"
                }
            },
            {
                "fullUrl": "urn:uuid:zorg-bij-jou-service-center",
                "resource": {
                    "resourceType": "Organization",
                    "id": "zorg-bij-jou-service-center",
                    "identifier": [
                        {
                            "system": "http://fhir.nl/fhir/NamingSystem/ura",
                            "value": "URA-001"
                        }
                    ],
                    "name": "Zorg Bij Jou - Medisch Service Center"
                },
                "request": {
                    "method": "POST",
                    "url": "Organization",
                    "ifNoneExist": "identifier=http://fhir.nl/fhir/NamingSystem/ura|URA-001"
                }
            },
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
                            "family": "Smith",
                            "given": [
                                "Jane",
                                "John"
                            ]
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
                            "line": ["456 Main Street"],
                            "city": "Worktown",
                            "state": "State",
                            "postalCode": "54321",
                            "country": "Country"
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
                        "reference": "urn:uuid:practitioner-1",
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
            },
            {
                "fullUrl": "urn:uuid:stantonius",
                "resource": {
                    "resourceType": "Organization",
                    "id": "StAntonius",
                    "identifier": [
                        {
                            "system": "http://fhir.nl/fhir/NamingSystem/ura",
                            "value": "URA-002"
                        }
                    ],
                    "name": "St. Antonius"
                },
                "request": {
                    "method": "POST",
                    "url": "Organization",
                    "ifNoneExist": "identifier=http://fhir.nl/fhir/NamingSystem/ura|URA-002"
                }
            },
            {
                "fullUrl": "urn:uuid:condition-1",
                "resource": {
                    "resourceType": "Condition",
                    "id": "condition-1",
                    "identifier": [
                        {
                            "system": "http://fhir.nl/fhir/NamingSystem/condition-identifier",
                            "value": "condition-001"
                        }
                    ],
                    "clinicalStatus": {
                        "coding": [
                            {
                                "system": "http://terminology.hl7.org/CodeSystem/condition-clinical",
                                "code": "active",
                                "display": "Active"
                            }
                        ]
                    },
                    "verificationStatus": {
                        "coding": [
                            {
                                "system": "http://terminology.hl7.org/CodeSystem/condition-ver-status",
                                "code": "confirmed",
                                "display": "Confirmed"
                            }
                        ]
                    },
                    "category": [
                        {
                            "coding": [
                                {
                                    "system": "http://terminology.hl7.org/CodeSystem/condition-category",
                                    "code": "problem-list-item",
                                    "display": "Problem List Item"
                                }
                            ]
                        }
                    ],
                    "code": {
                        "coding": [
                            {
                                "system": "http://snomed.info/sct",
                                "code": "13645005",
                                "display": "Chronic obstructive lung disease (disorder)"
                            }
                        ],
                        "text": "Chronische obstructieve longaandoening (aandoening)"
                    },
                    "subject": {
                        "type": "Patient",
                        "reference": "urn:uuid:patient-1",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/bsn",
                            "value": patientBsn
                        }
                    },
                    "recorder": {
                        "type": "PractitionerRole",
                        "reference": "urn:uuid:practitionerrole-1",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/uzi",
                            "value": "uzi-001"
                        }
                    }
                },
                "request": {
                    "method": "POST",
                    "url": "Condition"
                }
            },
            {
                "fullUrl": "urn:uuid:servicerequest-1",
                "resource": {
                    "resourceType": "ServiceRequest",
                    "id": "servicerequest-1",
                    "status": "draft",
                    "intent": "order",
                    "subject": {
                        "type": "Patient",
                        "reference": "urn:uuid:patient-1",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/bsn",
                            "value": patientBsn
                        }
                    },
                    "requester": {
                        "type": "Organization",
                        "display": "St. Antonius",
                        "reference": "urn:uuid:stantonius",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/ura",
                            "value": "URA-002"
                        }
                    },
                    "performer": [
                        {
                            "type": "Organization",
                            "display": "Zorg Bij Jou - Medisch Service Center",
                            "reference": "urn:uuid:zorg-bij-jou-service-center",
                            "identifier": {
                                "system": "http://fhir.nl/fhir/NamingSystem/ura",
                                "value": "URA-001"
                            }
                        }
                    ],
                    "code": {
                        "coding": [{
                            "system": "http://snomed.info/sct",
                            "code": "719858009",
                            "display": "monitoren via telegeneeskunde (regime/therapie)"
                        }]
                    },
                    "reasonReference": [
                        {
                            "type": "Condition",
                            "display": "Chronische obstructieve longaandoening (aandoening)",
                            "reference": "urn:uuid:condition-1",
                            "identifier": {
                                "system": "http://fhir.nl/fhir/NamingSystem/condition-identifier",
                                "value": "condition-001"
                            }
                        }
                    ]
                },
                "request": {
                    "method": "POST",
                    "url": "ServiceRequest"
                }
            }
        ]
    };
}
