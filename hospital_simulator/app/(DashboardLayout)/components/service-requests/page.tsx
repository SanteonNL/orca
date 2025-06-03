import React from 'react';
import CreateServiceRequestDialog from './create-service-request-dialog';
import ServiceRequestTable from './service-request-table';
import {Bundle, Patient, ServiceRequest} from 'fhir/r4';

type Input = {
    patient: Patient
}

export default async function Page(props: Input) {
    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }
    const patient = props.patient;

    const response = await fetch(`${process.env.FHIR_BASE_URL}/ServiceRequest?patient=Patient/${patient.id!!}&_count=500`, {
        cache: 'no-store',
        headers: {
            "Cache-Control": "no-cache"
        }
    });

    console.log(`Fetched SRs, status: ${response.status}`);
    if (!response.ok) {
        const errorText = await response.text();
        console.error('Failed to fetch service requests: ', errorText);
        throw new Error('Failed to fetch service requests: ' + errorText);
    }

    const serviceRequestsData = await response.json() as Bundle<ServiceRequest>
    console.log(`Found [${serviceRequestsData.total}] ServiceRequest resources`);
    const serviceRequests = serviceRequestsData.entry ?? [];


    let rows = serviceRequests.map((entry: any) => {
        const serviceRequest = entry.resource;
        const patientIdentifier = serviceRequest.subject ? serviceRequest.subject.identifier.value : ""
        const patientName = patient?.name && patient.name[0] ? patient.name[0].text : patientIdentifier;
        const reasonRef = serviceRequest.reasonReference?.[0].display || "unknown";

        return {
            id: serviceRequest.id,
            lastUpdated: new Date(serviceRequest.meta.lastUpdated),
            title: serviceRequest.code.coding[0].display,
            patient: patientName,
            status: serviceRequest.status,
            patientId: patient ? `Patient/${patient.id}` : patientIdentifier,
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
