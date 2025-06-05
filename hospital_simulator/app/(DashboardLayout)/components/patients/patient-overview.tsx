import React from 'react';
import CreatePatientDialog from './create-patient-dialog';
import PatientTable from './patient-table';
import {Bundle, Identifier, Patient} from 'fhir/r4';

export default async function PatientOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    const searchResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient/_search`, {
        method: 'POST',
        cache: 'no-store',
        headers: {
            "Cache-Control": "no-cache",
            "Content-Type": "application/x-www-form-urlencoded"
        },
        body: new URLSearchParams({
            "_count": "500"
        })
    });
    if (!searchResponse.ok) {
        const errorText = await searchResponse.text();
        console.error('Failed to fetch patients: ', errorText);
        throw new Error('Failed to fetch patients: ' + errorText);
    }
    const searchSet = await searchResponse.json() as Bundle<Patient>;
    // filter out Patients that have an extension, those were made through the CPS and are duplicates.
    // This is due to Demo EHR not having its own FHIR server on local dev (for lower resource consumption).
    const searchSetEntries = (searchSet.entry || []).filter(entry => {
        return entry.resource && (!entry.resource.extension || entry.resource.extension.length === 0)
    });
    console.log(`Found [${searchSetEntries.length}] Patient resources`);
    let patients = searchSetEntries.map(entry => entry.resource as Patient) || []

    const rows = patients.map((patient: Patient) => {
        return {
            id: patient.identifier?.find((identifier: Identifier) => identifier.system === "http://fhir.nl/fhir/NamingSystem/bsn")!!.value!!,
            primaryIdentifier: patient.identifier?.find((identifier: Identifier) => identifier.system === "http://fhir.nl/fhir/NamingSystem/bsn")!!,
            name: patient.name?.[0]!!,
            gender: patient.gender!!,
        }
    })
    return (
        <div>
            <CreatePatientDialog/>
            <PatientTable rows={rows}/>
        </div>
    );
}
