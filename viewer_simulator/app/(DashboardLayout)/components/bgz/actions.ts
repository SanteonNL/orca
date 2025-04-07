'use server';
import {
    CarePlan,
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
    ServiceRequest,
} from 'fhir/r4';

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
        serviceRequests,
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
            ['ServiceRequest', 'status=active'],
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
        serviceRequests: serviceRequests as ServiceRequest[],
    };
}

async function fetchBgzData(
    name: string,
    resourceType: string,
    query: string,
    carePlan: CarePlan,
) {
    const requestUrl = `${process.env.FHIR_AGGREGATE_URL}/${resourceType}/_search`;
    const xSCPContext = `${process.env[`${name}_CAREPLANSERVICE_URL`]}/CarePlan/${carePlan.id}`;
    console.log(
        `Sending request to ${requestUrl} with X-Scp-Context: ${xSCPContext}`,
    );

    const response = await fetch(requestUrl, {
        method: 'POST',
        body: query,
        headers: {
            Authorization: `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            'X-Scp-Context': xSCPContext,
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
