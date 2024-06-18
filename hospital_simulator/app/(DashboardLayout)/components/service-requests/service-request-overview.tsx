import React from 'react';
import CreateServiceRequestDialog from './create-service-request-dialog';
import ServiceRequestTable from './service-request-table';

export default async function ServiceRequestOverview() {

    if (!process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER) {
        console.error('NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined');
        return <>NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined</>;
    }

    let rows = [];

    try {
        const response = await fetch(`${process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER}/ServiceRequest?_include=ServiceRequest:subject&_count=500`, {
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

        const serviceRequestsData = await response.json();
        console.log(`Found [${serviceRequestsData.total}] ServiceRequest resources`);

        if (serviceRequestsData.total > 0) {
            const serviceRequests = serviceRequestsData.entry.filter((entry: any) => entry.resource.resourceType === 'ServiceRequest');
            const idToPatientMap = serviceRequestsData.entry
                .filter((entry: any) => entry.resource.resourceType === 'Patient')
                .reduce((acc: any, patient: any) => {
                    const resource = patient.resource;
                    const patientName = resource.name && resource.name[0] ? resource.name[0].text : 'Unknown';
                    acc[resource.id] = patient.resource;
                    return acc;
                }, {});

            rows = serviceRequests.map((entry: any) => {
                const serviceRequest = entry.resource;
                const patientId = serviceRequest.subject.reference.split('/').pop();
                const patient = idToPatientMap[patientId]
                const patientName = patient.name && patient.name[0] ? patient.name[0].text : serviceRequest.subject.reference;

                return {
                    id: serviceRequest.id,
                    lastUpdated: new Date(serviceRequest.meta.lastUpdated),
                    title: serviceRequest.code.coding[0].display,
                    patient: patientName,
                    status: serviceRequest.status,
                    patientId: patient.id
                };
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
