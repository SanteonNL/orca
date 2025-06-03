import React from 'react';
import CreatePatientDialog from './create-patient-dialog';
import PatientTable from './patient-table';
import {Bundle, Identifier, Patient} from 'fhir/r4';

export default async function PatientOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }
    try {
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
        console.log(`Found [${searchSet.entry?.length}] Patient resources`);
        const patients = searchSet.entry?.map(entry => entry.resource) as Patient[] || [];
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
    } catch (error) {
        console.error('Error occurred while fetching patients:', error);
    }
}
