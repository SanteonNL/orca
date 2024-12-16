import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';
import { Bundle, Task } from 'fhir/r4';

export default async function AcceptedTaskOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    let rows: any[] = [];

    try {
        let requestHeaders = new Headers();
        requestHeaders.set("Cache-Control", "no-cache")
        if (process.env.FHIR_AUTHORIZATION_TOKEN) {
            requestHeaders.set("Authorization", "Bearer " + process.env.FHIR_AUTHORIZATION_TOKEN);
        }
        requestHeaders.set("Content-Type", "application/x-www-form-urlencoded");
        const response = await fetch(`${process.env.FHIR_BASE_URL}/Task/_search`, {
            method: 'POST',
            cache: 'no-store',
            headers: requestHeaders,
            body: new URLSearchParams({
                '_sort': '-_lastUpdated',
                '_count': '100'
            })
        });

        if (!response.ok) {
            const errorText = await response.text();
            console.error('Failed to fetch tasks: ', errorText);
            throw new Error('Failed to fetch tasks: ' + errorText);
        }

        const taskData = await response.text();
        const responseBundle = JSON.parse(taskData) as Bundle;
        const { entry } = responseBundle
        console.log(`Found [${entry?.length}] Task resources`);

        if (entry?.length) {
            rows = entry.map((entry: any) => {
                const task = entry.resource as Task;
                const bsn = task.for?.identifier?.value || "Unknown";

                //TODO: An optional improvement would be to fetch & cache the task.requester by identifier if the display is not set
                return {
                    id: task.id,
                    requesterUra: task.requester?.identifier?.value ?? "Unknown",
                    requesterName: task.requester?.display ?? "Unknown",
                    performerUra: task.owner?.identifier?.value ?? "Unknown",
                    performerName: task.owner?.display ?? "Unknown",
                    isSubtask: !!task.partOf,
                    patientBsn: bsn,
                    serviceRequest: task.focus?.display ?? "Unknown",
                    condition: task?.reasonCode?.text ?? task?.reasonCode?.coding?.[0].display ?? "",
                    status: task.status,
                    lastUpdated: task.meta?.lastUpdated ? new Date(task.meta.lastUpdated) : new Date(),
                    task: task
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
