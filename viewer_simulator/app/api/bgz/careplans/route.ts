import Client from 'fhir-kit-client';
import { Bundle, CarePlan } from 'fhir/r4';
import { NextRequest, NextResponse } from 'next/server';

import { getBsn } from '@/utils/fhirUtils';

export async function GET(req: NextRequest) {
    const name = req.nextUrl.searchParams.get('name');
    const baseUrl = process.env[`${name}_CAREPLANSERVICE_URL`];

    if (!baseUrl) {
        throw new Error(
            'Missing $NAME_CAREPLANSERVICE_URL environment variable',
        );
    }

    try {
        const client = new Client({
            baseUrl,
            bearerToken: process.env[`${name}_BEARER_TOKEN`],
        });

        const response = (await client.search({
            resourceType: 'CarePlan',
        })) as Bundle;

        const rows =
            response.entry?.map((entry) => {
                const carePlan = entry.resource as CarePlan;

                // Find the careteam, first as a contained resource, otherwise as a referenced resource that has been notified
                const careTeam = carePlan.contained?.find(
                    (resource) => resource.resourceType === 'CareTeam',
                );

                if (!careTeam) {
                    console.warn(
                        `No CareTeam found for CarePlan/${carePlan.id}`,
                    );
                }

                return {
                    id: careTeam?.id || 'Unknown',
                    bsn: getBsn(carePlan),
                    category:
                        carePlan.category?.[0]?.coding
                            ?.map((coding) => coding.display)
                            .join(', ') ?? 'Unknown',
                    carePlan,
                    careTeam,
                    status: carePlan.status,
                    lastUpdated: carePlan.meta?.lastUpdated
                        ? new Date(carePlan.meta.lastUpdated)
                        : new Date(),
                    careTeamMembers:
                        careTeam?.participant
                            ?.map((participant: any) => {
                                const display = participant.member.display
                                    ? participant.member.display + ' '
                                    : '';
                                const ura = `(URA ${participant.member.identifier?.value || 'Unknown'})`;
                                return display + ura;
                            })
                            .join(', ') ?? 'Unknown',
                };
            }) || [];

        return NextResponse.json(rows);
    } catch (error) {
        if (
            error instanceof Error &&
            'digest' in error &&
            error.digest === 'DYNAMIC_SERVER_USAGE'
        ) {
            console.error('Error occurred while fetching careplans:', error);
        } else {
            throw error;
        }
    }
}
