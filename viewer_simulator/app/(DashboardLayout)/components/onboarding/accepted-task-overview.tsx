import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';

export default async function AcceptedTaskOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    let rows = [];

    try {
        const response = await fetch(`${process.env.FHIR_BASE_URL}/Task`, {
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

        if (taskData?.total > 0) {
            const tasks = taskData.entry

            rows = tasks.map((entry: any) => {
                const task = entry.resource;
                const bsn = task.for.identifier.value
                const serviceRequest = task.contained?.find((contained: any) => contained.resourceType === "ServiceRequest")

                console.log(task.basedOn[0].display)
                return {
                    id: task.id,
                    hospitalUra: serviceRequest.requester.identifier.value,
                    hospitalName: serviceRequest.requester.display,
                    patientBsn: bsn || "Unknown",
                    careplan: task.basedOn[0].display,
                    status: task.status,
                    lastUpdated: new Date(task.meta.lastUpdated),
                    task: task
                };
            });
        }

        console.log(rows)
    } catch (error) {
        console.error('Error occurred while fetching tasks:', error);
    }

    return (
        <EnrolledTaskTable rows={rows} />
    );
}
