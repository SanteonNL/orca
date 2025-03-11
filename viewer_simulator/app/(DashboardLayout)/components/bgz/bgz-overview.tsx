import React from 'react';
import { BundleEntry, CarePlan } from 'fhir/r4';
import CarePlanTable from './bgz-careplan-table';
import { getBsn } from '@/utils/fhirUtils';
import { headers } from 'next/headers'
import { getNotificationBundles } from '@/app/api/delivery/storage';

export default async function BgzOverview() {

    if (!process.env.FHIR_BASE_URL) {
        console.error('FHIR_BASE_URL is not defined');
        return <>FHIR_BASE_URL is not defined</>;
    }

    // prevent ssr from pre-rendering the page, as it won't be able to fetch resources from process.env.FHIR_BASE_URL
    const headersList = headers()

    let rows: any[] = [];

    try {
        // Get bundles from internal storage, join all entries
        const notificationBundles = await getNotificationBundles();
        let entries = notificationBundles.flatMap(bundle => bundle.entry || []);

        //map all the resources to their reference as it contains CarePlans, Patients, Tasks and CareTeams
        const resourceMap = entries?.reduce((map, entry: BundleEntry) => {
            const resource = entry.resource;
            map.set(`${resource?.resourceType}/${resource?.id}`, resource);
            return map;
        }, new Map<string, any>());


        rows = entries?.filter((entries) => entries.resource?.resourceType === "CarePlan")
            .map((entry: any) => entry.resource as CarePlan)
            .map((carePlan: CarePlan) => {

                // Find the careteam, first as a contained resource, otherwise as a referenced resource that has been notified
                const careTeam =
                    carePlan.contained?.find((resource: any) => resource.resourceType === "CareTeam") ??
                    (carePlan.careTeam?.[0]?.reference ? resourceMap?.get(carePlan.careTeam[0].reference) : undefined);

                if (!careTeam) {
                    console.warn(`No CareTeam found for CarePlan/${carePlan.id}`);
                }

                return {
                    id: careTeam?.id || "Unknown",
                    bsn: getBsn(carePlan),
                    category: carePlan.category?.[0]?.coding?.map(coding => coding.display).join(', ') ?? "Unknown",
                    carePlan,
                    careTeam,
                    status: carePlan.status,
                    lastUpdated: carePlan.meta?.lastUpdated ? new Date(carePlan.meta.lastUpdated) : new Date(),
                    careTeamMembers: careTeam?.participant?.map((participant: any) => {
                        const display = participant.member.display ? participant.member.display + ' ' : '';
                        const ura = `(URA ${participant.member.identifier?.value || 'Unknown'})`;
                        return display + ura;
                    }).join(', ') ?? "Unknown"
                };
            }) || [];
    } catch (error) {
        if (error instanceof Error && 'digest' in error && error.digest === 'DYNAMIC_SERVER_USAGE') {
            console.error('Error occurred while fetching tasks:', error);
        } else {

            throw error;
        }
    }

    return (
        <CarePlanTable rows={rows} />
    );
}