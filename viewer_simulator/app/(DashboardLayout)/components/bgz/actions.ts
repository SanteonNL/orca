"use server"
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
    ProcedureRequest
} from "fhir/r3";
import {CarePlan, CareTeam} from "fhir/r4";

export async function getBgzData(carePlan: CarePlan, careTeam: CareTeam) {

    const [appointments, patients, conditions, coverages, consents, observations, immunizations, immunizationRecommendations, deviceRequests, deviceUseStatements, encounters, flags, medicationRequests, nutritionOrders, procedures, procedureRequests] = await Promise.all([
        fetchBgzData('Appointment', 'status=booked,pending,proposed', carePlan),
        fetchBgzData('Patient', '_include=Patient:general-practitioner', carePlan),
        fetchBgzData('Condition', '', carePlan),
        fetchBgzData('Coverage', '', carePlan),
        fetchBgzData('Consent', 'category=http://snomed.info/sct|11291000146105', carePlan),
        fetchBgzData('Observation', 'code=http://snomed.info/sct%7C228366006', carePlan),
        fetchBgzData('Immunization', 'status=completed', carePlan),
        fetchBgzData('ImmunizationRecommendation', '', carePlan),
        fetchBgzData('DeviceRequest', 'status=active&_include=DeviceRequest:device', carePlan),
        fetchBgzData('DeviceUseStatement', '_include=DeviceUseStatement:device', carePlan),
        fetchBgzData('Encounter', 'class=http://hl7.org/fhir/v3/ActCode|IMP,http://hl7.org/fhir/v3/ActCode|ACUTE,http://hl7.org/fhir/v3/ActCode|NONAC', carePlan),
        fetchBgzData('Flag', '', carePlan),
        fetchBgzData('MedicationRequest', 'category=http://snomed.info/sct%7C16076005&_include=MedicationRequest:medication', carePlan),
        fetchBgzData('NutritionOrder', '', carePlan),
        fetchBgzData('Procedure', 'category=http://snomed.info/sct%7C387713003', carePlan),
        fetchBgzData('ProcedureRequest', 'status=active', carePlan),
    ]);

    //Important: MUST match the bgzStore synthax
    return {
        appointments: appointments as Appointment[],
        patient: patients?.[0] as Patient | undefined,
        conditions: conditions as Condition[],
        coverages: coverages as Coverage[],
        consents: consents as Consent[],
        observations: observations as Observation[],
        immunizations: immunizations as Immunization[],
        immunizationRecommendations: immunizationRecommendations as ImmunizationRecommendation[],
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

async function fetchBgzData(resourceType: string, query: string, carePlan: CarePlan) {
    const requestUrl = `${process.env.FHIR_AGGREGATE_URL}/${resourceType}/_search`;
    const xSCPContext = `${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`;
    console.log(`Sending request to ${requestUrl} with X-Scp-Context: ${xSCPContext}`);

    const response = await fetch(requestUrl, {
        method: 'POST',
        body: query,
        headers: {
            "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            "X-Scp-Context": xSCPContext,
            "Content-Type": "application/x-www-form-urlencoded",
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch ${requestUrl}: ${response.statusText}`);
        // throw new Error(`Failed to fetch ${resourceType}: ${response.statusText}`);
    }

    const result = await response.json() as Bundle

    console.log(`Received ${result.total} ${resourceType} entries`);

    return result.entry
        ?.filter((entry) => entry.resource?.resourceType === resourceType)
        ?.map((entry) => entry.resource);
}