"use client"
import * as React from 'react';
import {useState} from 'react';
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import {IconPlus} from '@tabler/icons-react';
import {Alert, Grid, MenuItem, Select} from '@mui/material';
import {useRouter} from 'next/navigation';
import {Coding, Organization, Patient, ServiceRequest} from 'fhir/r4';
import {getLocalOrganization, getTaskPerformerOrganization} from "@/utils/config";
import {AdapterDayjs} from "@mui/x-date-pickers/AdapterDayjs";
import {LocalizationProvider} from "@mui/x-date-pickers";
import 'dayjs/locale/nl';

type ServiceRequestDetails = {
    patient: Patient
    conditionCode: Coding
}

interface Props {
    patient: Patient
}

const CreateServiceRequestDialog: React.FC<Props> = ({patient}) => {
    const [open, setOpen] = React.useState(false);
    const [patientConditionCode, setPatientConditionCode] = useState<Coding>(supportedConditions[0])

    const [error, setError] = useState<string>()
    const router = useRouter()

    const handleClickOpen = () => {
        setOpen(true);
    };

    const handleClose = () => {
        setOpen(false);
    };

    const createServiceRequest = async () => {
        const requester = await getLocalOrganization()
        const performer = await getTaskPerformerOrganization()
        if (!patientConditionCode) {
            setError("Please select a condition")
            return
        }
        const details: ServiceRequestDetails = {
            conditionCode: patientConditionCode,
            patient: patient
        }
        const bundle = createServiceRequestBundle(details, requester, performer)

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
            <Button sx={{position: 'absolute', top: '10px', right: '10px'}} variant="contained"
                    onClick={handleClickOpen}>
                <IconPlus/>
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
                            <Grid size={{xs: 12}}>
                                <Alert severity="error">Something went wrong: {error}</Alert>
                            </Grid>
                        )}
                        <Grid size={{xs: 12}}>
                            <DialogContentText>
                                Create a new Service Request for the patient.
                            </DialogContentText>
                        </Grid>
                        <Grid size={{xs: 12}} sx={{mt: 2}}>
                            <Select fullWidth value="tele-monitoring">
                                <MenuItem value="tele-monitoring">Telemonitoring</MenuItem>
                            </Select>
                        </Grid>
                        <Grid size={{xs: 12}}>
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
const supportedConditions: Array<Coding> = [
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

function createServiceRequestBundle(requestDetails: ServiceRequestDetails,
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
                "fullUrl": "urn:uuid:performer",
                "resource": performer,
                "request": {
                    "method": "POST",
                    "url": "Organization",
                    "ifNoneExist": `identifier=${performerIdentifier.system}|${performerIdentifier.value}`
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
                        "coding": [requestDetails.conditionCode],
                        "text": requestDetails.conditionCode.display
                    },
                    "subject": {
                        "type": "Patient",
                        "reference": "Patient/" + requestDetails.patient.id!!,
                        "identifier": {
                            "system": requestDetails.patient.identifier!![0].system ?? "",
                            "value": requestDetails.patient.identifier!![0].value ?? ""
                        }
                    },
                    "recorder": {
                        "type": "PractitionerRole",
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
                        "reference": "Patient/" + requestDetails.patient.id!!,
                        "identifier": {
                            "system": requestDetails.patient.identifier!![0].system ?? "",
                            "value": requestDetails.patient.identifier!![0].value ?? ""
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
                            "display": "Thuismonitoring"
                        }],
                    },
                    "reasonReference": [
                        {
                            "type": "Condition",
                            "display": requestDetails.conditionCode.display,
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
