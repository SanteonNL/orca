import React from 'react';
import { Bundle, BundleEntry, CarePlan, Task } from 'fhir/r4';
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
        const notificationBundles = getNotificationBundles();
        let entries = notificationBundles.flatMap(bundle => bundle.entry || []);

        // TODO: Remove this once the notification from ehr to viewer is working correctly in dev and test environments
        if (entries.length === 0) {
            let requestHeaders = new Headers();
            requestHeaders.set("Cache-Control", "no-cache")
            if (process.env.FHIR_AUTHORIZATION_TOKEN) {
                requestHeaders.set("Authorization", "Bearer " + process.env.FHIR_AUTHORIZATION_TOKEN);
            }
            requestHeaders.set("Content-Type", "application/x-www-form-urlencoded");
            const response = await fetch(`${process.env.FHIR_BASE_URL}/CarePlan/_search`, {
                method: 'POST',
                headers: requestHeaders,
                body: new URLSearchParams({
                    '_sort': '-_lastUpdated',
                    '_count': '100',
                    '_include': 'CarePlan:care-team'
                })
            });

            if (!response.ok) {
                const errorText = await response.text();
                console.error('Failed to fetch tasks: ', errorText);
                throw new Error('Failed to fetch tasks: ' + errorText);
            }

            const responseBundle = await response.json() as Bundle;
            entries = responseBundle.entry || [];
            console.log(`Found [${entries?.length}] CarePlan resources`);
        }

        //map all the resources to their reference as it contains CarePlans, Patients, Tasks and CareTeams
        const resourceMap = entries?.reduce((map, entry: BundleEntry) => {
            const resource = entry.resource;
            map.set(`${resource?.resourceType}/${resource?.id}`, resource);
            return map;
        }, new Map<string, any>());


        rows = entries?.filter((entries) => entries.resource?.resourceType === "CarePlan")
            .map((entry: any) => entry.resource as CarePlan)
            .map((carePlan: CarePlan) => {
                const careTeam = carePlan.careTeam?.[0]?.reference ? resourceMap?.get(carePlan.careTeam[0].reference) : undefined;

                if (!careTeam) {
                    console.warn(`No CareTeam found for CarePlan/${carePlan.id}`);
                }

                return {
                    id: careTeam.id,
                    bsn: getBsn(carePlan),
                    category: carePlan.category?.[0]?.coding?.map(coding => coding.display).join(', ') ?? "Unknown",
                    carePlan,
                    careTeam,
                    status: carePlan.status,
                    lastUpdated: carePlan.meta?.lastUpdated ? new Date(carePlan.meta.lastUpdated) : new Date(),
                    careTeamMembers: careTeam.participant?.map((participant: any) => {
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