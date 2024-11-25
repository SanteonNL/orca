"use server"
import { Bundle, CarePlan, CareTeam } from "fhir/r4";

export async function getBgzData(carePlan: CarePlan, careTeam: CareTeam) {

    //TODO: Use a Promise.all for all of the bgz requests
    const response = await fetch(`${process.env.FHIR_AGGREGATE_URL}/Patient?_include=Patient:general-practitioner`, {
        headers: {
            "Authorization": `Bearer ${process.env.FHIR_AUTHORIZATION_TOKEN}`,
            "X-Scp-Context": `${process.env.FHIR_BASE_URL}/CarePlan/${carePlan.id}`
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch Patient: ${response.statusText}`);
        return;
    }

    return await response.json() as Bundle;
}