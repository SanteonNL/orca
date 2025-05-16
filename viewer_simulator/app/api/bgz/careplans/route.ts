import {Bundle, CarePlan} from 'fhir/r4';
import {NextRequest, NextResponse} from 'next/server';

import {getBsn} from '@/utils/fhirUtils';
import {getCarePlanServiceBaseURLs, getORCABearerToken, getORCAExternalFHIRBaseURL} from "@/utils/config";

export async function GET(req: NextRequest) {
    const name = req.nextUrl.searchParams.get('name');
    if (!name) {
        return NextResponse.json(
            {error: 'Missing name query parameter'},
            {status: 400},
        );
    }
    let carePlans = [] as CarePlan[];
    for (const cpsURL of getCarePlanServiceBaseURLs(name)) {
        carePlans = carePlans.concat(await fetchCarePlans(name, cpsURL));
    }
    const rows = carePlans.map((carePlan) => {
        // Find the careteam, first as a contained resource, otherwise as a referenced resource that has been notified
        const careTeam = carePlan.contained?.find(
            (resource) => resource.resourceType === 'CareTeam',
        );
        if (!careTeam) {
            console.warn(`No CareTeam found for CarePlan/${carePlan.id}`);
            return undefined
        }
        return {
            id: careTeam?.id || 'Unknown',
            bsn: getBsn(carePlan),
            category:
                carePlan.category?.[0]?.coding
                    ?.map((coding) => coding.display)
                    .join(', ') ?? 'Unknown',
            carePlan,
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
    }).filter((row) => row !== undefined);
    return NextResponse.json(rows);
}

async function fetchCarePlans(name: string, cpsBaseURL: string) {
    const httpRequestURL = `${getORCAExternalFHIRBaseURL(name)}/CarePlan/_search`;
    const orcaBearerToken = getORCABearerToken(name);

    let bundle = {} as Bundle;
    try {
        const resp = await fetch(httpRequestURL, {
            method: 'POST',
            headers: {
                Authorization: `Bearer ${orcaBearerToken}`,
                'Content-Type': 'x-www-form-urlencoded',
                'X-Scp-Fhir-Url': cpsBaseURL,
            },
            body: `_count=200`
        });
        if (!resp.ok) {
            throw new Error(`Failed to fetch data: ${httpRequestURL} (X-Scp-Fhir-Url: ${cpsBaseURL}), status: ${resp.status}`);
        }
        bundle = await resp.json() as Bundle;
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
    const carePlans = bundle.entry?.map((entry) => entry.resource as CarePlan) || [];
    // Set the meta.source to the ORCA base URL, so it can be used as X-Scp-Context later (the ORCA FHIR proxy only sets meta.source for the top-level resource).
    return carePlans.map(carePlan => {
        carePlan.meta = {
            ...carePlan.meta,
            source: `${cpsBaseURL}/CarePlan/${carePlan.id}`,
        };
        return carePlan;
    })
}