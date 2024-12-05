"use server"
import { Appointment, Bundle, Condition, Consent, Coverage, DeviceRequest, DeviceUseStatement, Encounter, Flag, Immunization, ImmunizationRecommendation, MedicationRequest, NutritionOrder, Observation, Patient, Procedure, ProcedureRequest } from "fhir/r3";
import { CarePlan, CareTeam } from "fhir/r4";

export async function getBgzData(carePlan: CarePlan, careTeam: CareTeam) {

    const [appointments, patients, conditions, coverages, consents, observations, immunizations, immunizationRecommendations, deviceRequests, deviceUseStatements, encounters, flags, medicationRequests, nutritionOrders, procedures, procedureRequests] = await Promise.all([
        fetchBgzData('Appointment?status=booked,pending,proposed', 'Appointment', carePlan),
        fetchBgzData('Patient?_include=Patient:general-practitioner', 'Patient', carePlan),
        fetchBgzData('Condition', 'Condition', carePlan),
        fetchBgzData('Coverage', 'Coverage', carePlan),
        fetchBgzData('Consent?category=http://snomed.info/sct|11291000146105', 'Consent', carePlan),
        fetchBgzData('Observation?code=http://snomed.info/sct%7C228366006', 'Observation', carePlan),
        fetchBgzData('Immunization?status=completed', 'Immunization', carePlan),
        fetchBgzData('ImmunizationRecommendation', 'ImmunizationRecommendation', carePlan),
        fetchBgzData('DeviceRequest?status=active&_include=DeviceRequest:device', 'DeviceRequest', carePlan),
        fetchBgzData('DeviceUseStatement?_include=DeviceUseStatement:device', 'DeviceUseStatement', carePlan),
        fetchBgzData('Encounter?class=http://hl7.org/fhir/v3/ActCode|IMP,http://hl7.org/fhir/v3/ActCode|ACUTE,http://hl7.org/fhir/v3/ActCode|NONAC', 'Encounter', carePlan),
        fetchBgzData('Flag', 'Flag', carePlan),
        fetchBgzData('MedicationRequest?category=http://snomed.info/sct%7C16076005&_include=MedicationRequest:medication', 'MedicationRequest', carePlan),
        fetchBgzData('NutritionOrder', 'NutritionOrder', carePlan),
        fetchBgzData('Procedure?category=http://snomed.info/sct%7C387713003', 'Procedure', carePlan),
        fetchBgzData('ProcedureRequest?status=active', 'ProcedureRequest', carePlan),
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

async function fetchBgzData(path: string, resourceType: string, carePlan: CarePlan) {

    console.log(`Sending request to ${process.env.FHIR_AGGREGATE_URL}/${path} with X-Scp-Context: ${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`);

    const response = await fetch(`${process.env.FHIR_AGGREGATE_URL}/${path}`, {
        headers: {
            "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            "X-Scp-Context": `${process.env.ORCA_CAREPLANCONTRIBUTOR_CAREPLANSERVICE_URL}/CarePlan/${carePlan.id}`
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch ${path}: ${response.statusText}`);
        // throw new Error(`Failed to fetch ${resourceType}: ${response.statusText}`);
    }

    const result = await response.json() as Bundle

    console.log(`Received ${result.total} ${resourceType} entries`);

    return result.entry
        ?.filter((entry) => entry.resource?.resourceType === resourceType)
        ?.map((entry) => entry.resource);
}