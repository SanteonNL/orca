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
import {Alert, Grid, InputLabel, MenuItem, Select} from '@mui/material';
import {useRouter} from 'next/navigation';
import {AdapterDayjs} from "@mui/x-date-pickers/AdapterDayjs";
import {DesktopDatePicker, LocalizationProvider} from "@mui/x-date-pickers";
import dayjs from "dayjs";
import 'dayjs/locale/nl';
import {Organization} from "fhir/r4";
import {getLocalOrganization} from "@/utils/config";

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
}


const CreatePatientDialog: React.FC = () => {
    const [open, setOpen] = React.useState(false);
    const [patientFirstName, setPatientFirstName] = useState('')
    const [patientLastName, setPatientLastName] = useState('')
    const [patientBsn, setPatientBsn] = useState(generatePatientBsn())
    const [patientEmail, setPatientEmail] = useState('')
    const [patientPhone, setPatientPhone] = useState(generatePatientPhone())
    const [patientGender, setPatientGender] = useState<AdministrativeGender>("unknown")
    const [patientBirthdate, setPatientBirthdate] = useState(dayjs('1980-01-15'))
    const [selfSetPatientEmail, setSelfSetPatientEmail] = useState(false)

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
    }, [patientFirstName, patientLastName, selfSetPatientEmail]);

    const createPatient = async () => {
        const localOrg = await getLocalOrganization()
        let firstName = patientFirstName || "John";
        let lastName = patientLastName || "Doe";
        const patientDetails: PatientDetails = {
            phone: patientPhone,
            gender: patientGender,
            bsn: patientBsn,
            email: patientEmail || generatePatientEmail(firstName, lastName),
            firstName: firstName,
            lastName: lastName,
            birthdate: patientBirthdate,
            postcode: patientPostcode,
            address: patientAdress,
            city: patientCity,
        }
        const bundle = createPatientBundle(patientDetails, localOrg)

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
                <DialogTitle id="alert-dialog-title">New Patient</DialogTitle>
                <DialogContent>
                    <Grid container spacing={2}>
                        {error && (
                            <Grid size={{xs: 12}}>
                                <Alert severity="error">Something went wrong: {error}</Alert>
                            </Grid>
                        )}
                        <Grid size={{xs: 12}}>
                            <DialogContentText>
                                Create a patient.
                            </DialogContentText>
                        </Grid>
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 6}}>
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
                        <Grid size={{xs: 12, md: 12}}>
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
                        <Grid size={{xs: 12, md: 8}}>
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
                        <Grid size={{xs: 12, md: 4}}>
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
                    </Grid>
                </DialogContent>
                <DialogActions>
                    <Button onClick={handleClose}>Cancel</Button>
                    <Button onClick={createPatient} autoFocus>
                        Create
                    </Button>
                </DialogActions>
            </Dialog>
        </LocalizationProvider>
    );
}

export default CreatePatientDialog

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
function test11Proef(bsn: string) {
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

function createPatientBundle(patient: PatientDetails, localOrg: Organization) {
    return {
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:patient-1",
                "resource": {
                    "resourceType": "Patient",
                    "extension": [
                        {
                            "url": "http://santeonnl.github.io/shared-care-planning/StructureDefinition/resource-creator",
                            "valueReference": {
                                "type": "Organization",
                                "identifier": {
                                    "system": localOrg.identifier?.[0].system || "",
                                    "value": localOrg.identifier?.[0].value || "",
                                }
                            }
                        }
                    ],
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
            }
        ]
    };
}
