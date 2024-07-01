import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';

export default async function AcceptedTaskOverview() {

    if (!process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER) {
        console.error('NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined');
        return <>NEXT_PUBLIC_FHIR_BASE_URL_DOCKER is not defined</>;
    }

    let rows = [];

    try {
        const response = await fetch(`${process.env.NEXT_PUBLIC_FHIR_BASE_URL_DOCKER}/Task?reasonCode=http%3A%2F%2Fsnomed.info%2Fsct%7C719858009&status=accepted`, {
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

            rows = tasks.map((entry: any) => {
                const task = entry.resource;
                console.log(JSON.stringify(task, undefined, 2))
                let forRef = task.for.reference
                if (forRef.indexOf('#') === 0) { //local reference
                    forRef = forRef.substring(1, forRef.length)
                }

                const patientResource = task.contained && task.contained.find((containedResource: any) => containedResource.id === forRef)
                const patientIdentifier = patientResource && patientResource.identifier[0]
                const patientName = patientResource && patientResource.name[0]

                return {
                    id: task.id,
                    hospitalUra: task.requester.identifier.value,
                    hospitalName: task.requester.display,
                    patientBsn: patientIdentifier?.value || "Unknown",
                    patientLastname: patientName?.given[0] || "Unknown",
                    patientSurname: patientName?.family || "Unknown",
                    condition: task.reasonCode.coding[0].display,
                    status: task.status,
                    lastUpdated: new Date(task.meta.lastUpdated),
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
