import React from 'react';
import CreatePatientDialog from './create-patient-dialog';
import PatientTable from './patient-table';
import {Bundle, Identifier, Patient} from 'fhir/r4';
import CreatePractitioner from "@/app/(DashboardLayout)/practitioner";
import {DefaultAzureCredential} from '@azure/identity';

export default async function PatientOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }
    await CreatePractitioner();

    // Get authentication token for Azure FHIR if not in local environment
    let token: string | null = null;
    const fhirUrl = process.env.FHIR_BASE_URL || '';
    if (!fhirUrl.includes('localhost') && !fhirUrl.includes('fhirstore')) {
        try {
            const credential = new DefaultAzureCredential();
            const tokenResponse = await credential.getToken(`${fhirUrl}/.default`);
            token = tokenResponse.token;
        } catch (error) {
            console.error('Azure authentication failed:', error);
            throw error;
        }
    }

    const headers: HeadersInit = {
        "Cache-Control": "no-cache",
        "Content-Type": "application/x-www-form-urlencoded"
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    const searchResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient/_search`, {
        method: 'POST',
        cache: 'no-store',
        headers: headers,
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
            resourceId: patient.id!!,
            primaryIdentifier: patient.identifier?.find((identifier: Identifier) => identifier.system === "http://fhir.nl/fhir/NamingSystem/bsn")!!,
            name: patient.name?.[0]!!,
            gender: patient.gender!!,
            lastUpdated: new Date(patient.meta?.lastUpdated || 0),
        }
    })
    return (
        <div>
            <CreatePatientDialog/>
            <PatientTable rows={rows}/>
        </div>
    );
}
