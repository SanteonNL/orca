import React from 'react';
import EnrolledTaskTable from './enrolled-task-table';
import { Bundle, PractitionerRole, Task } from 'fhir/r4';
import { headers } from 'next/headers'
import {getNotificationBundles} from "@/app/api/delivery/storage";

export default async function AcceptedTaskOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    // prevent ssr from pre-rendering the page, as it won't be able to fetch resources from process.env.FHIR_BASE_URL
    const headersList = headers()

    let rows: any[] = [];

    try {
        // Get bundles from internal storage, join all entries
        const notificationBundles = getNotificationBundles();
        let entries = notificationBundles.flatMap(bundle => bundle.entry || []);
        console.log(`Found [${entries?.length}] bundle resources`);

        if (entries?.length) {
            rows = entries.
            filter((entries) => entries.resource?.resourceType === "Task").
            map((entry: any) => {
                const task = entry.resource as Task;
                const bsn = task.for?.identifier?.value || "Unknown";

                const practitionerRole = task.contained?.find((resource: any) => resource.resourceType === "PractitionerRole") as PractitionerRole | undefined;
                let practitionerRoleIdentifiers
                if (practitionerRole) {
                    practitionerRoleIdentifiers = practitionerRole.identifier
                        ?.map((identifier: any) => `${identifier.system}|${identifier.value}`)
                        .join(', ') || "Unknown";
                }

                //TODO: An optional improvement would be to fetch & cache the task.requester by identifier if the display is not set
                return {
                    id: task.id,
                    requesterUra: task.requester?.identifier?.value ?? "Unknown",
                    requesterName: task.requester?.display ?? "Unknown",
                    practitionerRoleIdentifiers,
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
