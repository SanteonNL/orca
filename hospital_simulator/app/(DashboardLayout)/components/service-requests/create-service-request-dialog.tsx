"use client"
import * as React from 'react';
import {useEffect, useState} from 'react';
import Button from '@mui/material/Button';
import TextField from '@mui/material/TextField';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import {IconPlus} from '@tabler/icons-react';
import {Alert, Divider, Grid, InputLabel, MenuItem, Select} from '@mui/material';
import {useRouter} from 'next/navigation';
import {Coding, Organization} from 'fhir/r4';
import {getLocalOrganization, getTaskPerformerOrganization} from "@/utils/config";
import {AdapterDayjs} from "@mui/x-date-pickers/AdapterDayjs";
import {DesktopDatePicker, LocalizationProvider} from "@mui/x-date-pickers";
import dayjs from "dayjs";
import 'dayjs/locale/nl';

type AdministrativeGender = "male" | "female" | "unknown" | "other";

type PatientDetails = {
    firstName: string,
    lastName: string,
    bsn: string,
    email: string,
    phone: string,
    gender: AdministrativeGender,
    birthdate: dayjs.Dayjs,
    address: string,
    city: string,
    postcode: string,
    conditionCode: Coding
}


const CreateServiceRequestDialog: React.FC = () => {
    const [open, setOpen] = React.useState(false);
    const [patientFirstName, setPatientFirstName] = useState('')
    const [patientLastName, setPatientLastName] = useState('')
    const [patientBsn, setPatientBsn] = useState(generatePatientBsn())
    const [patientEmail, setPatientEmail] = useState('')
    const [patientPhone, setPatientPhone] = useState(generatePatientPhone())
    const [patientGender, setPatientGender] = useState<AdministrativeGender>("unknown")
    const [patientBirthdate, setPatientBirthdate] = useState(dayjs('1980-01-15'))
    const [selfSetPatientEmail, setSelfSetPatientEmail] = useState(false)
    const [patientConditionCode, setPatientConditionCode] = useState<Coding>(supportedConditions[0])

    const [patientAdress, setPatientAddress] = useState('Hoofdstraat 123')
    const [patientCity, setPatientCity] = useState('Meerdijkerveen')
    const [patientPostcode, setPatientPostcode] = useState('4321BA')

    const [error, setError] = useState<string>()
    const router = useRouter()

    const handleClickOpen = () => {
        setOpen(true);
    };

    const handleClose = () => {
        setOpen(false);
        setPatientFirstName('')
        setPatientLastName('')
        setPatientBsn(generatePatientBsn())
        setPatientPhone(generatePatientPhone())
        setPatientEmail('')
        setSelfSetPatientEmail(false)
    };

    useEffect(() => {
        if (patientFirstName && patientLastName && !selfSetPatientEmail) {
            setPatientEmail(generatePatientEmail(patientFirstName, patientLastName))
        }
    }, [patientFirstName, patientLastName]);

    const createServiceRequest = async () => {
        const requester = await getLocalOrganization()
        const performer = await getTaskPerformerOrganization()
        if (!patientConditionCode) {
            setError("Please select a condition")
            return
        }
        let firstName = patientFirstName || "John";
        let lastName = patientLastName || "Doe";
        const patientDetails: PatientDetails = {
            phone: patientPhone,
            gender: patientGender,
            bsn: patientBsn,
            email: patientEmail || generatePatientEmail(firstName, lastName),
            firstName: firstName,
            lastName: lastName,
            conditionCode: patientConditionCode,
            birthdate: patientBirthdate,
            postcode: patientPostcode,
            address: patientAdress,
            city: patientCity,
        }
        const bundle = createServiceRequestBundle(patientDetails, requester, performer)

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
        <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale="nl">
            <Button sx={{ position: 'absolute', top: '10px', right: '10px' }} variant="contained"
                onClick={handleClickOpen}>
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
                            <Grid size={{xs:12}}>
                                <Alert severity="error">Something went wrong: {error}</Alert>
                            </Grid>
                        )}
                        <Grid size={{xs:12}}>
                            <DialogContentText>
                                Create a new ServiceRequest for a new Patient. For demo purposes, we can only
                                create <i>Telemonitoring</i> ServiceRequests.
                            </DialogContentText>
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <TextField
                                autoFocus
                                required
                                value={patientFirstName}
                                label="Patient first name"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientFirstName(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <TextField
                                autoFocus
                                required
                                value={patientLastName}
                                label="Patient family name"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientLastName(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <TextField
                                autoFocus
                                required
                                type='email'
                                value={patientEmail}
                                label="Patient email"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setSelfSetPatientEmail(true)
                                    setPatientEmail(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <TextField
                                autoFocus
                                required
                                value={patientPhone}
                                type='tel'
                                label="Patient phone"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientPhone(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <TextField
                                autoFocus
                                required
                                value={patientBsn}
                                label="Patient BSN"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientBsn(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <InputLabel id="select-gender">Gender</InputLabel>
                            <Select
                                autoFocus
                                required
                                value={patientGender}
                                labelId="select-gender"
                                onChange={event => setPatientGender(event.target.value as AdministrativeGender)}
                                fullWidth
                                variant="standard"
                            >
                                <MenuItem value={"unknown"}>unknown</MenuItem>
                                <MenuItem value={"male"}>male</MenuItem>
                                <MenuItem value={"female"}>female</MenuItem>
                                <MenuItem value={"other"}>other</MenuItem>
                            </Select>
                        </Grid>
                        <Grid size={{xs:12, md:6}}>
                            <DesktopDatePicker
                                label="Birthdate"
                                value={patientBirthdate}
                                onChange={(newValue) => {
                                    if (newValue) {
                                        setPatientBirthdate(newValue)
                                    }
                                }}
                            />
                        </Grid>
                        <Grid size={{xs:12, md:12}}>
                            <TextField
                                autoFocus
                                required
                                value={patientAdress}
                                label="Patient address"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientAddress(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:8}}>
                            <TextField
                                autoFocus
                                required
                                value={patientCity}
                                label="Patient city"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientCity(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12, md:4}}>
                            <TextField
                                autoFocus
                                required
                                value={patientPostcode}
                                label="Patient postcode"
                                onChange={(event: React.ChangeEvent<HTMLInputElement>) => {
                                    setPatientPostcode(event.target.value);
                                }}
                                fullWidth
                                variant="standard"
                            />
                        </Grid>
                        <Grid size={{xs:12}} sx={{ mt: 2 }}>
                            <Select fullWidth value="tele-monitoring">
                                <MenuItem value="tele-monitoring">Telemonitoring</MenuItem>
                            </Select>
                        </Grid>
                        <Grid size={{xs:12}}>
                            <Select fullWidth value={patientConditionCode.code} onChange={(event) => {
                                const conditionCode = supportedConditions.find((condition) => condition.code === event.target.value)
                                if (conditionCode) {
                                    setPatientConditionCode(conditionCode)
                                }
                            }}>
                                {supportedConditions.map((condition) => (
                                    <MenuItem key={condition.code} value={condition.code}>{condition.display}</MenuItem>
                                ))}
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
        </LocalizationProvider>
    );
}

export default CreateServiceRequestDialog

// supportedConditions specifies the conditions that can be used in the ServiceRequest: COPD, Asthma, Heart failure
const supportedConditions : Array<Coding> = [
    {
        "system": "http://snomed.info/sct",
        "code": "84114007",
        "display": "Heart failure (disorder)"
    },
    {
        "system": "http://snomed.info/sct",
        "code": "195967001",
        "display": "Asthma (disorder)"
    },
    {
        "system": "http://snomed.info/sct",
        "code": "13645005",
        "display": "Chronic obstructive pulmonary disease (disorder)"
    }
]

function generatePatientEmail(firstName: string, lastName: string) {
    // remove characters not allowed in email addresses, concatenate first and lastname, add a random number
    // to ensure uniqueness
    return `${firstName.replace(/[^a-zA-Z0-9]/g, '')}.${lastName.replace(/[^a-zA-Z0-9]/g, '')}.${Math.floor(Math.random() * 1000)}@example.com`
}

function generatePatientBsn() {
    let prefix = "99999";
    // Ensure the tail is exactly 4 digits, pad with zeros if necessary.
    const tail = Math.floor(Math.random() * 10000).toString().padStart(4, '0');
    // Concatenate to form an 8-digit candidate.
    let bsn = prefix + tail;
    while (!test11Proef(bsn)) {
        const tail = Math.floor(Math.random() * 1000).toString().padStart(3, '0');
        bsn = prefix + tail;
    }
    return bsn
}

function generatePatientPhone() {
    // Some downstream systems might be sending actual messages to these phone numbers, so use a fixed, non-existing one.
    return '+31 6 12345678';
    // let prefix = "+31 6 ";
    // // Ensure the tail is exactly 4 digits, pad with zeros if necessary.
    // const tail = Math.floor(Math.random() * 100000000).toString().padStart(8, '0');
    // // Concatenate to form an 8-digit candidate.
    // let bsn = prefix + tail;
    // return bsn
}
/**
 * Nederlandse burgerservicenummers, de vroegere sofinummers, voldoen aan een variant van de elfproef.
 * In de elfproef is het laatste cijfer het controlecijfer. Bij burgerservicenummers wordt het laatste
 * getal met −1 vermenigvuldigd in plaats van met 1. Uitgaande van een nummer dat voldoet aan de elfproef
 * kan geen nieuw geldig nummer worden gegenereerd door één cijfer te veranderen of door twee cijfers te verwisselen.
 */
function test11Proef(bsn:string) {
    const reversed = bsn.split("").reverse().join("")
    let total = 0
    for (let i = 0; i < reversed.length; i++) {
        let weight = i + 1
        if (weight === 1) {
            weight = -1
        }
        const digit = parseInt(reversed[i])
        const sum = weight * digit
        total += sum
    }
    return total % 11 === 0
}

function createServiceRequestBundle(patient: PatientDetails,
                                    requester: Organization,
                                    performer: Organization) {

    if (requester.identifier?.length !== 1) {
        throw new Error("Requester must have exactly one identifier")
    }
    const requesterIdentifier = requester.identifier[0]
    if (performer.identifier?.length !== 1) {
        throw new Error("Performer must have exactly one identifier")
    }
    const performerIdentifier = performer.identifier[0]

    return {
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:patient-1",
                "resource": {
                    "extension": [
                        {
                            "url": "http://santeonnl.github.io/shared-care-planning/StructureDefinition/resource-creator",
                            "valueReference": {
                                "type": "Organization",
                                "identifier": {
                                    "system": requesterIdentifier.system,
                                    "value": requesterIdentifier.value
                                }
                            }
                        }
                    ],
                    "resourceType": "Patient",
                    "identifier": [
                        {
                            "use": "usual",
                            "system": "http://fhir.nl/fhir/NamingSystem/bsn",
                            "value": patient.bsn
                        }
                    ],
                    "name": [
                        {
                            "family": patient.lastName,
                            "given": [
                                patient.firstName,
                            ],
                            "text": `${patient.lastName}, ${patient.firstName}`
                        }
                    ],
                    "gender": patient.gender,
                    "birthDate": patient.birthdate.format("YYYY-MM-DD"),
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
                            "value": patient.phone,
                            "use": "home"
                        },
                        {
                            "system": "email",
                            "value": patient.email,
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
                "fullUrl": "urn:uuid:performer",
                "resource": performer,
                "request": {
                    "method": "POST",
                    "url": "Organization",
                    "ifNoneExist": `identifier=${performerIdentifier.system}|${performerIdentifier.value}`
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
                            "line": [patient.address],
                            "city": patient.city,
                            "postalCode": patient.postcode,
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
                "fullUrl": "urn:uuid:requester",
                "resource": requester,
                "request": {
                    "method": "POST",
                    "url": "Organization",
                    "ifNoneExist": `identifier=${requesterIdentifier.system}|${requesterIdentifier.value}`
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
                        "coding": [patient.conditionCode],
                        "text": patient.conditionCode.display
                    },
                    "subject": {
                        "type": "Patient",
                        "reference": "urn:uuid:patient-1",
                        "identifier": {
                            "system": "http://fhir.nl/fhir/NamingSystem/bsn",
                            "value": patient.bsn
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
                            "value": patient.bsn
                        }
                    },
                    "requester": {
                        "type": "Organization",
                        "display": `${requester.name}`,
                        "reference": "urn:uuid:requester",
                        "identifier": {
                            "system": `${requesterIdentifier.system}`,
                            "value": `${requesterIdentifier.value}`
                        }
                    },
                    "performer": [
                        {
                            "type": "Organization",
                            "display": `${performer.name}`,
                            "reference": "urn:uuid:performer",
                            "identifier": {
                                "system": `${performerIdentifier.system}`,
                                "value": `${performerIdentifier.value}`
                            }
                        }
                    ],
                    "code": {
                        "coding": [{
                            "system": "http://snomed.info/sct",
                            "code": "719858009",
                            "display": "Telehealth monitoring (regime/therapy)"
                        }],
                    },
                    "reasonReference": [
                        {
                            "type": "Condition",
                            "display": patient.conditionCode.display,
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
