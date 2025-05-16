'use server';
import {
    Appointment,
    Bundle,
    Condition,
    Consent,
    Coverage,
    DeviceRequest,
    DeviceUseStatement,
    Encounter,
    Flag,
    Immunization,
    ImmunizationRecommendation,
    MedicationRequest,
    NutritionOrder,
    Observation,
    Patient,
    Procedure,
    ProcedureRequest,
} from 'fhir/r3';
import {CarePlan} from 'fhir/r4';
import {careTeamFromCarePlan, identifierToToken} from "@/utils/fhirUtils";
import {getORCABearerToken, getORCAExternalFHIRBaseURL, getOwnIdentifier} from "@/utils/config";

export async function getBgzData(name: string, carePlan: CarePlan) {
    const [
        appointments,
        patients,
        conditions,
        coverages,
        consents,
        observations,
        immunizations,
        immunizationRecommendations,
        deviceRequests,
        deviceUseStatements,
        encounters,
        flags,
        medicationRequests,
        nutritionOrders,
        procedures,
        procedureRequests,
    ] = await Promise.all(
        [
            ['Appointment', 'status=booked,pending,proposed'],
            ['Patient', '_include=Patient:general-practitioner'],
            ['Condition', ''],
            ['Coverage', ''],
            ['Consent', 'category=http://snomed.info/sct|11291000146105'],
            ['Observation', 'code=http://snomed.info/sct%7C228366006'],
            ['Immunization', 'status=completed'],
            ['ImmunizationRecommendation', ''],
            ['DeviceRequest', 'status=active&_include=DeviceRequest:device'],
            ['DeviceUseStatement', '_include=DeviceUseStatement:device'],
            [
                'Encounter',
                'class=http://hl7.org/fhir/v3/ActCode|IMP,http://hl7.org/fhir/v3/ActCode|ACUTE,http://hl7.org/fhir/v3/ActCode|NONAC',
            ],
            ['Flag', ''],
            [
                'MedicationRequest',
                'category=http://snomed.info/sct%7C16076005&_include=MedicationRequest:medication',
            ],
            ['NutritionOrder', ''],
            ['Procedure', 'category=http://snomed.info/sct%7C387713003'],
            ['ProcedureRequest', 'status=active'],
        ].map(([resourceType, query]) =>
            fetchBgzData(name, resourceType, query, carePlan),
        ),
    );

    //Important: MUST match the bgzStore synthax
    return {
        appointments: appointments as Appointment[],
        patient: patients?.[0] as Patient | undefined,
        conditions: conditions as Condition[],
        coverages: coverages as Coverage[],
        consents: consents as Consent[],
        observations: observations as Observation[],
        immunizations: immunizations as Immunization[],
        immunizationRecommendations:
            immunizationRecommendations as ImmunizationRecommendation[],
        deviceRequests: deviceRequests as DeviceRequest[],
        deviceUseStatements: deviceUseStatements as DeviceUseStatement[],
        encounters: encounters as Encounter[],
        flags: flags as Flag[],
        medicationRequests: medicationRequests as MedicationRequest[],
        nutritionOrders: nutritionOrders as NutritionOrder[],
        procedures: procedures as Procedure[],
        procedureRequests: procedureRequests as ProcedureRequest[],
    };
}

async function fetchBgzData(
    name: string,
    resourceType: string,
    query: string,
    carePlan: CarePlan,
) {
    const careTeam = careTeamFromCarePlan(carePlan);
    if (!careTeam) {
        throw new Error("CarePlan doesn't contain a CareTeam");
    }

    // Filter out invalid identifiers, and our own identifier from the CareTeam participants
    const remotePartyIdentifiers = careTeam.participant?.filter(
        (participant) => participant.member?.identifier &&
            (participant.member?.identifier?.system && participant.member?.identifier?.value) &&
            (identifierToToken(participant.member?.identifier) !== identifierToToken(getOwnIdentifier(name)))
    );
    if (!remotePartyIdentifiers || remotePartyIdentifiers.length == 0) {
        throw new Error(`CareTeam.participants doesn't contain any (remote) queryable parties.`);
    }
    if (remotePartyIdentifiers.length > 1) {
        throw new Error(`CareTeam.participants contains multiple queryable parties, which is not supported.`);
    }
    const remotePartyIdentifier = remotePartyIdentifiers[0].member?.identifier!!;

    const requestUrl = `${getORCAExternalFHIRBaseURL(name)}/${resourceType}/_search`;
    if (!carePlan.meta?.source) {
        throw new Error(`CarePlan doesn't contain a meta.source`);
    }
    const xSCPContext = carePlan.meta.source

    console.log(
        `Sending request to ${requestUrl} (X-Scp-Context=${xSCPContext}, X-Scp-Entity-Identifier=${identifierToToken(remotePartyIdentifier)})`,
    );

    const response = await fetch(requestUrl, {
        method: 'POST',
        body: query,
        headers: {
            Authorization: `Bearer ${getORCABearerToken(name)}`,
            'X-Scp-Context': xSCPContext,
            'X-Scp-Entity-Identifier': identifierToToken(remotePartyIdentifier),
            'Content-Type': 'application/x-www-form-urlencoded',
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch ${requestUrl}: ${response.statusText}`);
        // throw new Error(`Failed to fetch ${resourceType}: ${response.statusText}`);
    }

    const result = (await response.json()) as Bundle;

    console.log(`Received ${result.total} ${resourceType} entries`);

    return result.entry
        ?.filter((entry) => entry.resource?.resourceType === resourceType)
        ?.map((entry) => entry.resource);
}
