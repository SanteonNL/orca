import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';

export default async function AcceptedTaskOverview() {

    if (!process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER) {
        console.error('NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined');
        return <>NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined</>;
    }

    let rows = [];

    try {
        const response = await fetch(`${process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER}/Task`, {
            cache: 'no-store',
            headers: {
                "Cache-Control": "no-cache"
            }
        });

        if (!response.ok) {
            const errorText = await response.text();
            console.error('Failed to fetch tasks: ', errorText);
            throw new Error('Failed to fetch tasks: ' + errorText);
        }

        const taskData = await response.json();
        console.log(`Found [${taskData.total}] Task resources`);

        if (taskData.total > 0) {
            const tasks = taskData.entry
            // const idToPatientMap = taskData.entry
            //     .filter((entry: any) => entry.resource.resourceType === 'Patient')
            //     .reduce((acc: any, patient: any) => {
            //         const resource = patient.resource;
            //         const patientName = resource.name && resource.name[0] ? resource.name[0].text : 'Unknown';
            //         acc[resource.id] = patient.resource;
            //         return acc;
            //     }, {});

            rows = tasks.map((entry: any) => {
                const task = entry.resource;
                // const patientId = task.subject.reference.split('/').pop();
                // const patient = idToPatientMap[patientId]
                // const patientName = patient.name && patient.name[0] ? patient.name[0].text : task.subject.reference;

                return {
                    id: task.id,
                    lastUpdated: new Date(task.meta.lastUpdated),
                    title: task.reasonCode.text,
                    owner: task.owner.display,
                    patient: task.for.display,
                    status: task.status,
                };
            });
        }
    } catch (error) {
        console.error('Error occurred while fetching tasks:', error);
    }

    return (
        <EnrolledTaskTable rows={rows} />
    );
}
