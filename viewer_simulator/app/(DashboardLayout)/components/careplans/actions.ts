"use server"

import { CarePlan, CareTeam } from "fhir/r4";

export async function getBgzData(carePlan: CarePlan, careTeam: CareTeam) {

    //TODO: Should fetch the BGZ data from all CareTeam members - for now it only queries the configured hospital
    // careTeam.participant?.forEach(async (participant) => {
    //     participant
    // });

    //TODO: Use a Promise.all for all of the bgz requests
    const response = await fetch(`${process.env.FHIR_HOSPITAL_URL}/Patient?_include=Patient:general-practitioner`, {
        headers: {
            "X-Scp-Context": `${process.env.FHIR_BASE_URL}/CarePlan/${carePlan.id}`
        },
    });

    if (!response.ok) {
        console.error(`Failed to fetch Patient: ${await response.text()}`);
        return;
    }

    return await response.json();
}