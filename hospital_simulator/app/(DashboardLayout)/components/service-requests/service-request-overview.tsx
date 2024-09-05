import React from 'react';
import CreateServiceRequestDialog from './create-service-request-dialog';
import ServiceRequestTable from './service-request-table';
import { Bundle, Identifier, Patient } from 'fhir/r4';

export default async function ServiceRequestOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    let rows = [];

    const getBsnIdentifier = (identifiers: Identifier[] | undefined): string | undefined => {
        const identifier = identifiers?.find(identifier => identifier.system === "http://fhir.nl/fhir/NamingSystem/bsn");
        return identifier?.value;
    };

    try {
        const response = await fetch(`${process.env.FHIR_BASE_URL}/ServiceRequest?_count=500`, {
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

        const patientsResponse = await fetch(`${process.env.FHIR_BASE_URL}/Patient?_count=500`, {
            cache: 'no-store',
            headers: {
                "Cache-Control": "no-cache"
            }
        });

        console.log(`Fetched Patients, status: ${patientsResponse.status}`);
        if (!patientsResponse.ok) {
            const errorText = await patientsResponse.text();
            console.error('Failed to fetch patients: ', errorText);
            throw new Error('Failed to fetch patients: ' + errorText);
        }

        const serviceRequestsData = await response.json();
        console.log(`Found [${serviceRequestsData.total}] ServiceRequest resources`);

        if (serviceRequestsData.total > 0) {
            const serviceRequests = serviceRequestsData.entry;

            const patientsBundle = await patientsResponse.json() as Bundle<Patient>;
            const idToPatientMap = (patientsBundle.entry || []).reduce((acc: { [key: string]: Patient }, patientBundleEntry) => {
                const patient = patientBundleEntry.resource as Patient;
                if (patient) {
                    const identifier = getBsnIdentifier(patient.identifier);
                    if (identifier) {
                        acc[identifier] = patient;
                    }
                }
                return acc;
            }, {});

            rows = serviceRequests.map((entry: any) => {
                const serviceRequest = entry.resource;
                const patient = serviceRequest.subject ? idToPatientMap[serviceRequest.subject.identifier.value] : undefined;
                const patientIdentifier = serviceRequest.subject ? serviceRequest.subject.identifier.value : ""
                const patientName = patient?.name && patient.name[0] ? patient.name[0].text : patientIdentifier;

                return {
                    id: serviceRequest.id,
                    lastUpdated: new Date(serviceRequest.meta.lastUpdated),
                    title: serviceRequest.code.coding[0].display,
                    patient: patientName,
                    status: serviceRequest.status,
                    patientId: patient ? `Patient/${patient.id}` : patientIdentifier
                }
            });
        }
    } catch (error) {
        console.error('Error occurred while fetching service requests:', error);
    }

    return (
        <div>
            <CreateServiceRequestDialog />
            <ServiceRequestTable rows={rows} />
        </div>
    );
}
