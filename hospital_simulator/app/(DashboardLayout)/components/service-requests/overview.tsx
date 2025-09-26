import React from 'react';
import CreateServiceRequestDialog from './create-service-request-dialog';
import ServiceRequestTable from './service-request-table';
import {Bundle, ServiceRequest} from 'fhir/r4';
import {ReadPatient} from "@/utils/fhir";
import {DefaultAzureCredential} from '@azure/identity';

type Input = {
    patientID: string
}

export default async function Overview(props: Input) {
    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

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
        "Cache-Control": "no-cache"
    };

    if (token) {
        headers['Authorization'] = `Bearer ${token}`;
    }

    const response = await fetch(`${process.env.FHIR_BASE_URL}/ServiceRequest?patient=Patient/${props.patientID}&_count=500`, {
        cache: 'no-store',
        headers: headers
    });

    console.log(`Fetched SRs, status: ${response.status}`);
    if (!response.ok) {
        const errorText = await response.text();
        console.error('Failed to fetch service requests: ', errorText);
        throw new Error('Failed to fetch service requests: ' + errorText);
    }

    const serviceRequestsData = await response.json() as Bundle<ServiceRequest>
    console.log(`Found [${serviceRequestsData.total}] ServiceRequest resources`);
    let serviceRequests = serviceRequestsData.entry ?? [];
    // filter out ServiceRequests that have an extension, those were made through the CPS and are duplicates.
    // This is due to Demo EHR not having its own FHIR server on local dev (for lower resource consumption).
    serviceRequests = serviceRequests.filter(entry => {
        return entry.resource && (!entry.resource.extension || entry.resource.extension.length === 0)
    });

   const patient = await ReadPatient(props.patientID);
    if (!patient) {
        return <>Patient not found: {props.patientID}</>;
    }

    let rows = serviceRequests.map(entry => {
        const serviceRequest = entry.resource as ServiceRequest;
        const patientIdentifier = serviceRequest.subject.identifier ? serviceRequest.subject.identifier.value : ""
        const patientName = patient.name && patient.name[0] ? patient.name[0].text : patientIdentifier;
        const reasonRef = serviceRequest.reasonReference?.[0].display || "unknown";

        return {
            id: serviceRequest.id,
            lastUpdated: new Date(serviceRequest.meta?.lastUpdated || 0),
            title: serviceRequest.code?.coding!![0].display || "N/A",
            patient: patientName,
            status: serviceRequest.status,
            patientId: `Patient/${patient.id}`,
            reasonReference: reasonRef,
        }
    });
    return (
        <div>
            <CreateServiceRequestDialog patient={patient}/>
            <ServiceRequestTable rows={rows}/>
        </div>
    );
}
