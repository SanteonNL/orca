import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';
import { Task } from 'fhir/r4';

export default async function AcceptedTaskOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    let rows = [];

    try {
        let requestHeaders = new Headers();
        requestHeaders.set("Cache-Control", "no-cache")
        if (process.env.FHIR_AUTHORIZATION_TOKEN) {
            requestHeaders.set("Authorization", "Bearer " + process.env.FHIR_AUTHORIZATION_TOKEN);
        }
        const response = await fetch(`${process.env.FHIR_BASE_URL}/Task`, {
            cache: 'no-store',
            headers: requestHeaders
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
                    careplan: task.basedOn?.[0]?.reference ? task.basedOn?.[0]?.reference : "Unknown",
                    status: task.status,
                    lastUpdated: task.meta?.lastUpdated ? new Date(task.meta.lastUpdated) : new Date(),
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
